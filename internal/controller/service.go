package controller

import (
	"github.com/PDOK/ogcapi-operator/api/v1alpha1"
	v2 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

func getBareService(ogcAPI v1.Object) *v2.Service {
	return &v2.Service{
		ObjectMeta: v1.ObjectMeta{
			Name:      ogcAPI.GetName(),
			Namespace: ogcAPI.GetNamespace(),
		},
	}
}

func (r *OGCAPIReconciler) mutateService(ogcAPI *v1alpha1.OGCAPI, service *v2.Service) error {
	labels := getLabels(ogcAPI)
	if err := setImmutableLabels(r.Client, service, labels); err != nil {
		return err
	}

	internalTrafficPolicy := v2.ServiceInternalTrafficPolicyCluster
	service.Spec = v2.ServiceSpec{
		Type:                  v2.ServiceTypeClusterIP,
		ClusterIP:             service.Spec.ClusterIP,
		ClusterIPs:            service.Spec.ClusterIPs,
		IPFamilyPolicy:        service.Spec.IPFamilyPolicy,
		IPFamilies:            service.Spec.IPFamilies,
		SessionAffinity:       v2.ServiceAffinityNone,
		InternalTrafficPolicy: &internalTrafficPolicy,
		Ports: []v2.ServicePort{
			{
				Name:       mainPortName,
				Protocol:   v2.ProtocolTCP,
				Port:       mainPortNr,
				TargetPort: intstr.FromInt32(mainPortNr),
			},
		},
		Selector: labels,
	}
	if err := ensureSetGVK(r.Client, service, service); err != nil {
		return err
	}
	return controllerruntime.SetControllerReference(ogcAPI, service, r.Scheme)
}
