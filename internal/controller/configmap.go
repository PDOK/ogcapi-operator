package controller

import (
	pdoknlv1alpha1 "github.com/PDOK/ogcapi-operator/api/v1alpha1"
	smoothoperatorutil "github.com/pdok/smooth-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	yaml "sigs.k8s.io/yaml/goyaml.v3"
)

// getBareConfigMap sets the base name for the configmap containing the config for the gokoala Deployment.
// A hash suffix is/should be added to the actual full ConfigMap later.
func getBareConfigMap(ogcAPI metav1.Object) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getBareDeployment(ogcAPI).GetName(),
			Namespace: ogcAPI.GetNamespace(),
		},
	}
}

func (r *OGCAPIReconciler) mutateConfigMap(ogcAPI *pdoknlv1alpha1.OGCAPI, configMap *corev1.ConfigMap) error {
	configMap.Labels = getObjectLabels(ogcAPI, configMap.Labels)

	configYaml, err := yaml.Marshal(ogcAPI.Spec.Service)
	if err != nil {
		return err
	}
	configMap.Immutable = new(true)
	configMap.Data = map[string]string{configFileName: string(configYaml)}

	if err := smoothoperatorutil.EnsureSetGVK(r.Client, configMap, configMap); err != nil {
		return err
	}
	if err := controllerruntime.SetControllerReference(ogcAPI, configMap, r.Scheme); err != nil {
		return err
	}
	return smoothoperatorutil.AddHashSuffix(configMap)
}
