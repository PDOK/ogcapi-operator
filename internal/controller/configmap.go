package controller

import (
	"github.com/PDOK/ogcapi-operator/api/v1alpha1"
	v2 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	yaml "sigs.k8s.io/yaml/goyaml.v3"
)

// getBareConfigMap sets the base name for the configmap containing the config for the gokoala Deployment.
// A hash suffix is/should be added to the actual full ConfigMap later.
func getBareConfigMap(ogcAPI v1.Object) *v2.ConfigMap {
	return &v2.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:      getBareDeployment(ogcAPI).GetName(),
			Namespace: ogcAPI.GetNamespace(),
		},
	}
}

func (r *OGCAPIReconciler) mutateConfigMap(ogcAPI *v1alpha1.OGCAPI, configMap *v2.ConfigMap) error {
	labels := getLabels(ogcAPI)
	if err := setImmutableLabels(r.Client, configMap, labels); err != nil {
		return err
	}

	configYaml, err := yaml.Marshal(ogcAPI.Spec.Service)
	if err != nil {
		return err
	}
	configMap.Immutable = boolPtr(true)
	configMap.Data = map[string]string{configFileName: string(configYaml)}

	if err := ensureSetGVK(r.Client, configMap, configMap); err != nil {
		return err
	}
	if err := controllerruntime.SetControllerReference(ogcAPI, configMap, r.Scheme); err != nil {
		return err
	}
	return addHashSuffix(configMap)
}
