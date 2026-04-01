package controller

import (
	"github.com/PDOK/ogcapi-operator/api/v1alpha1"
	controller2 "github.com/pdok/smooth-operator/pkg/util"
	v2 "k8s.io/api/policy/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

func getBarePodDisruptionBudget(obj v1.Object) *v2.PodDisruptionBudget {
	return &v2.PodDisruptionBudget{
		ObjectMeta: v1.ObjectMeta{
			Name:      obj.GetName() + "-" + controllerName,
			Namespace: obj.GetNamespace(),
		},
	}
}

func (r *OGCAPIReconciler) mutatePodDisruptionBudget(ogcAPI *v1alpha1.OGCAPI, podDisruptionBudget *v2.PodDisruptionBudget) error {
	labels := getLabels(ogcAPI)
	if err := controller2.SetImmutableLabels(r.Client, podDisruptionBudget, labels); err != nil {
		return err
	}

	matchLabels := controller2.CloneOrEmptyMap(labels)
	podDisruptionBudget.Spec = v2.PodDisruptionBudgetSpec{
		MaxUnavailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
		Selector: &v1.LabelSelector{
			MatchLabels: matchLabels,
		},
	}

	if err := controller2.EnsureSetGVK(r.Client, podDisruptionBudget, podDisruptionBudget); err != nil {
		return err
	}
	return controllerruntime.SetControllerReference(ogcAPI, podDisruptionBudget, r.Scheme)
}
