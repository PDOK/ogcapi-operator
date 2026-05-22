package controller

import (
	pdoknlv1alpha1 "github.com/PDOK/ogcapi-operator/api/v1alpha1"
	smoothoperatorutil "github.com/pdok/smooth-operator/pkg/util"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

func getBareHorizontalPodAutoscaler(ogcAPI metav1.Object) *autoscalingv2.HorizontalPodAutoscaler {
	return &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getBareDeployment(ogcAPI).GetName(),
			Namespace: ogcAPI.GetNamespace(),
		},
	}
}

func (r *OGCAPIReconciler) mutateHorizontalPodAutoscaler(ogcAPI *pdoknlv1alpha1.OGCAPI, hpa *autoscalingv2.HorizontalPodAutoscaler) error {
	hpa.Labels = getObjectLabels(ogcAPI, hpa.Labels)
	hpa.Spec = autoscalingv2.HorizontalPodAutoscalerSpec{
		ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       getBareDeployment(ogcAPI).GetName(),
		},
		MinReplicas: new(int32(2)),
		MaxReplicas: 4,
		Metrics: []autoscalingv2.MetricSpec{
			{
				Type: autoscalingv2.ResourceMetricSourceType,
				Resource: &autoscalingv2.ResourceMetricSource{
					Name: corev1.ResourceCPU,
					Target: autoscalingv2.MetricTarget{
						Type:               autoscalingv2.UtilizationMetricType,
						AverageUtilization: new(int32(80)),
					},
				},
			},
			{
				Type: autoscalingv2.ResourceMetricSourceType,
				Resource: &autoscalingv2.ResourceMetricSource{
					Name: corev1.ResourceMemory,
					Target: autoscalingv2.MetricTarget{
						Type:               autoscalingv2.UtilizationMetricType,
						AverageUtilization: new(int32(75)),
					},
				},
			},
		},
		Behavior: &autoscalingv2.HorizontalPodAutoscalerBehavior{
			ScaleDown: &autoscalingv2.HPAScalingRules{
				StabilizationWindowSeconds: new(int32(900)),
				Policies: []autoscalingv2.HPAScalingPolicy{
					{
						Type:          autoscalingv2.PodsScalingPolicy,
						Value:         1,
						PeriodSeconds: 300,
					},
				},
			},
			ScaleUp: &autoscalingv2.HPAScalingRules{
				StabilizationWindowSeconds: new(int32(300)),
				Policies: []autoscalingv2.HPAScalingPolicy{
					{
						Type:          autoscalingv2.PodsScalingPolicy,
						Value:         1,
						PeriodSeconds: 60,
					},
				},
			},
		},
	}
	if ogcAPI.HorizontalPodAutoscalerPatch() != nil {
		patchedSpec, err := smoothoperatorutil.StrategicMergePatch(&hpa.Spec, ogcAPI.HorizontalPodAutoscalerPatch())
		if err != nil {
			return err
		}
		hpa.Spec = *patchedSpec
	}
	if err := smoothoperatorutil.EnsureSetGVK(r.Client, hpa, hpa); err != nil {
		return err
	}
	return controllerruntime.SetControllerReference(ogcAPI, hpa, r.Scheme)
}
