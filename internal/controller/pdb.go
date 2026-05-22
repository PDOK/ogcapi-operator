package controller

import (
	pdoknlv1alpha1 "github.com/PDOK/ogcapi-operator/api/v1alpha1"
	smoothoperatorutil "github.com/pdok/smooth-operator/pkg/util"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

func getBarePodDisruptionBudget(obj metav1.Object) *policyv1.PodDisruptionBudget {
	return &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      obj.GetName() + "-" + controllerName,
			Namespace: obj.GetNamespace(),
		},
	}
}

func (r *OGCAPIReconciler) mutatePodDisruptionBudget(ogcAPI *pdoknlv1alpha1.OGCAPI, podDisruptionBudget *policyv1.PodDisruptionBudget) error {
	podDisruptionBudget.Labels = getObjectLabels(ogcAPI, podDisruptionBudget.Labels)

	podDisruptionBudget.Spec = policyv1.PodDisruptionBudgetSpec{
		MaxUnavailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
		Selector:       getLabelSelector(ogcAPI),
	}

	if err := smoothoperatorutil.EnsureSetGVK(r.Client, podDisruptionBudget, podDisruptionBudget); err != nil {
		return err
	}
	return controllerruntime.SetControllerReference(ogcAPI, podDisruptionBudget, r.Scheme)
}
