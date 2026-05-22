package controller

import (
	"strconv"

	pdoknlv1alpha1 "github.com/PDOK/ogcapi-operator/api/v1alpha1"
	smoothoperatorutil "github.com/pdok/smooth-operator/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

func getBareDeployment(ogcAPI metav1.Object) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: ogcAPI.GetName() + "-" + gokoalaName,
			// name might become too long. not handling here. will just fail on apply.
			Namespace: ogcAPI.GetNamespace(),
		},
	}
}

//nolint:funlen
func (r *OGCAPIReconciler) mutateDeployment(ogcAPI *pdoknlv1alpha1.OGCAPI, deployment *appsv1.Deployment, configMapName string) error {
	deployment.Labels = getObjectLabels(ogcAPI, deployment.Labels)

	podTemplateAnnotations := smoothoperatorutil.CloneOrEmptyMap(deployment.Spec.Template.GetAnnotations())
	podTemplateAnnotations[priorityAnnotation+"/"+gokoalaName] = "4"

	deployment.Spec.Selector = getLabelSelector(ogcAPI)

	deployment.Spec.MinReadySeconds = 0
	deployment.Spec.ProgressDeadlineSeconds = new(int32(600))
	deployment.Spec.Strategy = appsv1.DeploymentStrategy{
		Type: appsv1.RollingUpdateDeploymentStrategyType,
		RollingUpdate: &appsv1.RollingUpdateDeployment{
			MaxUnavailable: new(intstr.FromInt32(0)),
			MaxSurge:       new(intstr.FromInt32(2)),
		},
	}
	deployment.Spec.RevisionHistoryLimit = new(int32(1))

	// deployment.Spec.Replicas is controlled by the HPA
	// deployment.Spec.Paused is ignored to allow a manual intervention i.c.e.

	podTemplateSpec := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      getObjectLabels(ogcAPI, deployment.Spec.Template.Labels),
			Annotations: podTemplateAnnotations,
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{Name: gokoalaName + "-" + configName, VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapName,
					},
				}}},
				{Name: gokoalaName + "-" + gpkgCacheName, VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
			},
			Containers: []corev1.Container{
				{
					Name:            gokoalaName,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Ports: []corev1.ContainerPort{
						{Name: mainPortName, ContainerPort: mainPortNr},
						{Name: debugPortName, ContainerPort: debugPortNr},
					},
					Env: []corev1.EnvVar{
						{Name: configFileEnvVar, Value: srvDir + "/" + configName + "/" + configFileName},
						{Name: debugPortEnvVar, Value: strconv.Itoa(debugPortNr)},
						{Name: shutdownDelayEnvVar, Value: strconv.Itoa(30)},
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceMemory:           resource.MustParse("1Gi"),
							corev1.ResourceEphemeralStorage: resource.MustParse("50Mi"), // TODO other sane default in case of OGC API Features
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("500m"),
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: gokoalaName + "-" + configName, MountPath: srvDir + "/" + configName},
						{Name: gokoalaName + "-" + gpkgCacheName, MountPath: srvDir + "/" + gpkgCacheName},
					},
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path:   "/health",
								Port:   intstr.FromInt32(mainPortNr),
								Scheme: corev1.URISchemeHTTP,
							},
						},
						InitialDelaySeconds: 60,
						TimeoutSeconds:      5,
						PeriodSeconds:       10,
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path:   "/health",
								Port:   intstr.FromInt32(mainPortNr),
								Scheme: corev1.URISchemeHTTP,
							},
						},
						InitialDelaySeconds: 60,
						TimeoutSeconds:      5,
						PeriodSeconds:       10,
					},
				},
			},
		},
	}

	if ogcAPI.Spec.PodSpecPatch != nil {
		patchedPod, err := smoothoperatorutil.StrategicMergePatch(&podTemplateSpec.Spec, &ogcAPI.Spec.PodSpecPatch)
		if err != nil {
			return err
		}
		podTemplateSpec.Spec = *patchedPod
	}
	podTemplateSpec.Spec.Containers[0].Image = r.GokoalaImage
	deployment.Spec.Template = podTemplateSpec

	// set annotations for optional volume-operator, volume operator requires blob-prefix to be set
	if ogcAPI.VolumeOperatorSpec.BlobPrefix != "" {
		deployment = addVolumePopulatorToDeployment(deployment, ogcAPI)
	}

	if err := smoothoperatorutil.EnsureSetGVK(r.Client, deployment, deployment); err != nil {
		return err
	}
	return controllerruntime.SetControllerReference(ogcAPI, deployment, r.Scheme)
}

func addVolumePopulatorToDeployment(deployment *appsv1.Deployment, ogcAPI *pdoknlv1alpha1.OGCAPI) *appsv1.Deployment {
	hash := smoothoperatorutil.GenerateHashFromStrings([]string{
		ogcAPI.VolumeOperatorSpec.BlobPrefix,
		volumeMountPath,
		ogcAPI.VolumeOperatorSpec.StorageCapacity,
	})
	deployment.Annotations = smoothoperatorutil.CloneOrEmptyMap(deployment.Annotations)
	deployment.Annotations["volume-operator.pdok.nl/blob-prefix"] = ogcAPI.VolumeOperatorSpec.BlobPrefix
	deployment.Annotations["volume-operator.pdok.nl/volume-path"] = volumeMountPath
	deployment.Annotations["volume-operator.pdok.nl/storage-class"] = ogcAPI.VolumeOperatorSpec.StorageClass
	deployment.Annotations["volume-operator.pdok.nl/resource-suffix"] = hash
	deployment.Annotations["volume-operator.pdok.nl/storage-capacity"] = ogcAPI.VolumeOperatorSpec.StorageCapacity

	volume := corev1.Volume{
		Name: gokoalaName + "-clone",
		VolumeSource: corev1.VolumeSource{
			Ephemeral: &corev1.EphemeralVolumeSource{
				VolumeClaimTemplate: &corev1.PersistentVolumeClaimTemplate{
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						StorageClassName: &ogcAPI.VolumeOperatorSpec.StorageClass,
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse(ogcAPI.VolumeOperatorSpec.StorageCapacity),
							},
						},
						DataSource: &corev1.TypedLocalObjectReference{
							Kind: "PersistentVolumeClaim",
							Name: hash,
						},
					},
				},
			},
		},
	}

	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, volume)
	for i, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == gokoalaName {
			deployment.Spec.Template.Spec.Containers[i].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[i].VolumeMounts, corev1.VolumeMount{
				Name:      volume.Name,
				MountPath: volumeMountPath,
			})
		}
	}
	return deployment
}
