package controller

import (
	pdoknlv1alpha1 "github.com/PDOK/ogcapi-operator/api/v1alpha1"
	smoothoperatorutil "github.com/pdok/smooth-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

func getBareService(ogcAPI metav1.Object) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ogcAPI.GetName(),
			Namespace: ogcAPI.GetNamespace(),
		},
	}
}

func (r *OGCAPIReconciler) mutateService(ogcAPI *pdoknlv1alpha1.OGCAPI, service *corev1.Service) error {
	service.Labels = getObjectLabels(ogcAPI, service.Labels)

	service.Spec = corev1.ServiceSpec{
		Type:                  corev1.ServiceTypeClusterIP,
		ClusterIP:             service.Spec.ClusterIP,
		ClusterIPs:            service.Spec.ClusterIPs,
		IPFamilyPolicy:        service.Spec.IPFamilyPolicy,
		IPFamilies:            service.Spec.IPFamilies,
		SessionAffinity:       corev1.ServiceAffinityNone,
		InternalTrafficPolicy: new(corev1.ServiceInternalTrafficPolicyCluster),
		Ports: []corev1.ServicePort{
			{
				Name:       mainPortName,
				Protocol:   corev1.ProtocolTCP,
				Port:       mainPortNr,
				TargetPort: intstr.FromInt32(mainPortNr),
			},
		},
		Selector: getLabelSelector(ogcAPI).MatchLabels,
	}
	if err := smoothoperatorutil.EnsureSetGVK(r.Client, service, service); err != nil {
		return err
	}
	return controllerruntime.SetControllerReference(ogcAPI, service, r.Scheme)
}
