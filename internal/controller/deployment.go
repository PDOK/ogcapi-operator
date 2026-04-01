package controller

import (
	"strconv"

	"github.com/PDOK/ogcapi-operator/api/v1alpha1"
	controller2 "github.com/pdok/smooth-operator/pkg/util"
	v2 "k8s.io/api/apps/v1"
	v3 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

func getBareDeployment(ogcAPI v1.Object) *v2.Deployment {
	return &v2.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name: ogcAPI.GetName() + "-" + gokoalaName,
			// name might become too long. not handling here. will just fail on apply.
			Namespace: ogcAPI.GetNamespace(),
		},
	}
}

//nolint:funlen
func (r *OGCAPIReconciler) mutateDeployment(ogcAPI *v1alpha1.OGCAPI, deployment *v2.Deployment, configMapName string) error {
	labels := getLabels(ogcAPI)
	if err := setImmutableLabels(r.Client, deployment, labels); err != nil {
		return err
	}

	podTemplateAnnotations := cloneOrEmptyMap(deployment.Spec.Template.GetAnnotations())
	podTemplateAnnotations[priorityAnnotation+"/"+gokoalaName] = "4"

	matchLabels := cloneOrEmptyMap(labels)
	deployment.Spec.Selector = &v1.LabelSelector{
		MatchLabels: matchLabels,
	}

	deployment.Spec.MinReadySeconds = 0
	deployment.Spec.ProgressDeadlineSeconds = int32Ptr(600)
	deployment.Spec.Strategy = v2.DeploymentStrategy{
		Type: v2.RollingUpdateDeploymentStrategyType,
		RollingUpdate: &v2.RollingUpdateDeployment{
			MaxUnavailable: intOrStrIntPtr(0),
			MaxSurge:       intOrStrIntPtr(2),
		},
	}
	deployment.Spec.RevisionHistoryLimit = int32Ptr(1)

	// deployment.Spec.Replicas is controlled by the HPA
	// deployment.Spec.Paused is ignored to allow a manual intervention i.c.e.

	podTemplateSpec := v3.PodTemplateSpec{
		ObjectMeta: v1.ObjectMeta{
			Labels:      matchLabels,
			Annotations: podTemplateAnnotations,
		},
		Spec: v3.PodSpec{
			Volumes: []v3.Volume{
				{Name: gokoalaName + "-" + configName, VolumeSource: v3.VolumeSource{ConfigMap: &v3.ConfigMapVolumeSource{
					LocalObjectReference: v3.LocalObjectReference{
						Name: configMapName,
					},
				}}},
				{Name: gokoalaName + "-" + gpkgCacheName, VolumeSource: v3.VolumeSource{EmptyDir: &v3.EmptyDirVolumeSource{}}},
			},
			Containers: []v3.Container{
				{
					Name:            gokoalaName,
					ImagePullPolicy: v3.PullIfNotPresent,
					Ports: []v3.ContainerPort{
						{Name: mainPortName, ContainerPort: mainPortNr},
						{Name: debugPortName, ContainerPort: debugPortNr},
					},
					Env: []v3.EnvVar{
						{Name: configFileEnvVar, Value: srvDir + "/" + configName + "/" + configFileName},
						{Name: debugPortEnvVar, Value: strconv.Itoa(debugPortNr)},
						{Name: shutdownDelayEnvVar, Value: strconv.Itoa(30)},
					},
					Resources: v3.ResourceRequirements{
						Limits: v3.ResourceList{
							v3.ResourceMemory:           resource.MustParse("1Gi"),
							v3.ResourceEphemeralStorage: resource.MustParse("50Mi"), // TODO other sane default in case of OGC API Features
						},
						Requests: v3.ResourceList{
							v3.ResourceCPU: resource.MustParse("500m"),
						},
					},
					VolumeMounts: []v3.VolumeMount{
						{Name: gokoalaName + "-" + configName, MountPath: srvDir + "/" + configName},
						{Name: gokoalaName + "-" + gpkgCacheName, MountPath: srvDir + "/" + gpkgCacheName},
					},
					LivenessProbe: &v3.Probe{
						ProbeHandler: v3.ProbeHandler{
							HTTPGet: &v3.HTTPGetAction{
								Path:   "/health",
								Port:   intstr.FromInt32(mainPortNr),
								Scheme: v3.URISchemeHTTP,
							},
						},
						InitialDelaySeconds: 60,
						TimeoutSeconds:      5,
						PeriodSeconds:       10,
					},
					ReadinessProbe: &v3.Probe{
						ProbeHandler: v3.ProbeHandler{
							HTTPGet: &v3.HTTPGetAction{
								Path:   "/health",
								Port:   intstr.FromInt32(mainPortNr),
								Scheme: v3.URISchemeHTTP,
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
		patchedPod, err := strategicMergePatch(&podTemplateSpec.Spec, &ogcAPI.Spec.PodSpecPatch)
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

	if err := ensureSetGVK(r.Client, deployment, deployment); err != nil {
		return err
	}
	return controllerruntime.SetControllerReference(ogcAPI, deployment, r.Scheme)
}

func addVolumePopulatorToDeployment(deployment *v2.Deployment, ogcAPI *v1alpha1.OGCAPI) *v2.Deployment {
	hash := controller2.GenerateHashFromStrings([]string{
		ogcAPI.VolumeOperatorSpec.BlobPrefix,
		volumeMountPath,
		ogcAPI.VolumeOperatorSpec.StorageCapacity,
	})
	deployment.Annotations = cloneOrEmptyMap(deployment.Annotations)
	deployment.Annotations["volume-operator.pdok.nl/blob-prefix"] = ogcAPI.VolumeOperatorSpec.BlobPrefix
	deployment.Annotations["volume-operator.pdok.nl/volume-path"] = volumeMountPath
	deployment.Annotations["volume-operator.pdok.nl/storage-class"] = ogcAPI.VolumeOperatorSpec.StorageClass
	deployment.Annotations["volume-operator.pdok.nl/resource-suffix"] = hash
	deployment.Annotations["volume-operator.pdok.nl/storage-capacity"] = ogcAPI.VolumeOperatorSpec.StorageCapacity

	volume := v3.Volume{
		Name: gokoalaName + "-clone",
		VolumeSource: v3.VolumeSource{
			Ephemeral: &v3.EphemeralVolumeSource{
				VolumeClaimTemplate: &v3.PersistentVolumeClaimTemplate{
					Spec: v3.PersistentVolumeClaimSpec{
						AccessModes: []v3.PersistentVolumeAccessMode{
							v3.ReadWriteOnce,
						},
						StorageClassName: &ogcAPI.VolumeOperatorSpec.StorageClass,
						Resources: v3.VolumeResourceRequirements{
							Requests: v3.ResourceList{
								v3.ResourceStorage: resource.MustParse(ogcAPI.VolumeOperatorSpec.StorageCapacity),
							},
						},
						DataSource: &v3.TypedLocalObjectReference{
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
			deployment.Spec.Template.Spec.Containers[i].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[i].VolumeMounts, v3.VolumeMount{
				Name:      volume.Name,
				MountPath: volumeMountPath,
			})
		}
	}
	return deployment
}
