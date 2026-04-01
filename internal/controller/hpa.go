package controller

import (
	"github.com/PDOK/ogcapi-operator/api/v1alpha1"
	controller2 "github.com/pdok/smooth-operator/pkg/util"
	v2 "k8s.io/api/autoscaling/v2"
	v3 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

func getBareHorizontalPodAutoscaler(ogcAPI v1.Object) *v2.HorizontalPodAutoscaler {
	return &v2.HorizontalPodAutoscaler{
		ObjectMeta: v1.ObjectMeta{
			Name:      getBareDeployment(ogcAPI).GetName(),
			Namespace: ogcAPI.GetNamespace(),
		},
	}
}

func (r *OGCAPIReconciler) mutateHorizontalPodAutoscaler(ogcAPI *v1alpha1.OGCAPI, hpa *v2.HorizontalPodAutoscaler) error {
	labels := getLabels(ogcAPI)
	if err := setImmutableLabels(r.Client, hpa, labels); err != nil {
		return err
	}
	hpa.Spec = v2.HorizontalPodAutoscalerSpec{
		ScaleTargetRef: v2.CrossVersionObjectReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       getBareDeployment(ogcAPI).GetName(),
		},
		MinReplicas: int32Ptr(2),
		MaxReplicas: 4,
		Metrics: []v2.MetricSpec{
			{
				Type: v2.ResourceMetricSourceType,
				Resource: &v2.ResourceMetricSource{
					Name: v3.ResourceCPU,
					Target: v2.MetricTarget{
						Type:               v2.UtilizationMetricType,
						AverageUtilization: int32Ptr(80),
					},
				},
			},
			{
				Type: v2.ResourceMetricSourceType,
				Resource: &v2.ResourceMetricSource{
					Name: v3.ResourceMemory,
					Target: v2.MetricTarget{
						Type:               v2.UtilizationMetricType,
						AverageUtilization: int32Ptr(75),
					},
				},
			},
		},
		Behavior: &v2.HorizontalPodAutoscalerBehavior{
			ScaleDown: &v2.HPAScalingRules{
				StabilizationWindowSeconds: int32Ptr(900),
				Policies: []v2.HPAScalingPolicy{
					{
						Type:          v2.PodsScalingPolicy,
						Value:         1,
						PeriodSeconds: 300,
					},
				},
			},
			ScaleUp: &v2.HPAScalingRules{
				StabilizationWindowSeconds: int32Ptr(300),
				Policies: []v2.HPAScalingPolicy{
					{
						Type:          v2.PodsScalingPolicy,
						Value:         1,
						PeriodSeconds: 60,
					},
				},
			},
		},
	}
	if ogcAPI.HorizontalPodAutoscalerPatch() != nil {
		patchedSpec, err := controller2.StrategicMergePatch(&hpa.Spec, ogcAPI.HorizontalPodAutoscalerPatch())
		if err != nil {
			return err
		}
		hpa.Spec = *patchedSpec
	}
	if err := ensureSetGVK(r.Client, hpa, hpa); err != nil {
		return err
	}
	return controllerruntime.SetControllerReference(ogcAPI, hpa, r.Scheme)
}
