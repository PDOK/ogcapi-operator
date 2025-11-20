/*
MIT License

Copyright (c) 2024 Publieke Dienstverlening op de Kaart

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package v1alpha1

import (
	gokoalaconfig "github.com/PDOK/gokoala/config"
	smoothoperatormodel "github.com/pdok/smooth-operator/model"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HorizontalPodAutoscalerPatch - copy of autoscalingv2.HorizontalPodAutoscalerSpec without ScaleTargetRef
// This way we don't have to specify the scaleTargetRef field in the CRD.
type HorizontalPodAutoscalerPatch struct {
	MinReplicas *int32                                         `json:"minReplicas,omitempty"`
	MaxReplicas *int32                                         `json:"maxReplicas,omitempty"`
	Metrics     []autoscalingv2.MetricSpec                     `json:"metrics,omitempty"`
	Behavior    *autoscalingv2.HorizontalPodAutoscalerBehavior `json:"behavior,omitempty"`
}

// OGCAPISpec defines the desired state of OGCAPI
// +kubebuilder:pruning:PreserveUnknownFields
// ^^ unknown fields are allowed (but ignored and not maintained when marshalling) for backwards compatibility
type OGCAPISpec struct {
	Service gokoalaconfig.Config `json:"service"`
	//+kubebuilder:validation:Type=object
	//+kubebuilder:validation:Schemaless
	//+kubebuilder:pruning:PreserveUnknownFields
	// Optional strategic merge patch for the pod in the deployment. E.g. to patch the resources or add extra env vars.
	PodSpecPatch *corev1.PodSpec `json:"podSpecPatch,omitempty"`

	// Optional specification for the HorizontalAutoscaler
	HorizontalPodAutoscalerPatch *HorizontalPodAutoscalerPatch `json:"horizontalPodAutoscalerPatch,omitempty"`

	// Optional list of URLs where the service can be reached
	// By default only the spec.service.baseUrl is used
	IngressRouteURLs smoothoperatormodel.IngressRouteURLs `json:"ingressRouteUrls,omitempty"`
}

type BlobDownloadOptions struct {
	BlockSize       string `json:"blockSize,omitempty"`
	BlobConcurrency string `json:"blobConcurrency,omitempty"`
}

// VolumeOperatorSpec defines the way the volumes are managed
type VolumeOperatorSpec struct {
	// The prefix to use for the blob storage container (the location of the data)
	BlobPrefix string `json:"blobPrefix"`

	// The storage capacity to request for the volumes created by the volume operator, min: 1Gi
	StorageCapacity string `json:"storageCapacity,omitempty"`

	// The storage class to use for the volumes created by the volume operator, defaults to zrs-managed-premium
	StorageClass string `json:"storageClass,omitempty"`

	BlobDownloadOptions BlobDownloadOptions `json:"blobDownloadOptions,omitempty"`
}

// OGCAPI is the Schema for the ogcapis API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=pdok
// +kubebuilder:printcolumn:name="ReadyPods",type=integer,JSONPath=`.status.podSummary[0].ready`
// +kubebuilder:printcolumn:name="DesiredPods",type=integer,JSONPath=`.status.podSummary[0].total`
// +kubebuilder:printcolumn:name="ReconcileStatus",type=string,JSONPath=`.status.conditions[?(@.type == "Reconciled")].reason`
type OGCAPI struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec               OGCAPISpec                         `json:"spec,omitempty"`
	Status             smoothoperatormodel.OperatorStatus `json:"status,omitempty"`
	VolumeOperatorSpec VolumeOperatorSpec                 `json:"volumeOperatorSpec,omitempty"`
}

func (ogcapi *OGCAPI) OperatorStatus() *smoothoperatormodel.OperatorStatus {
	return &ogcapi.Status
}

func (ogcapi *OGCAPI) HorizontalPodAutoscalerPatch() *HorizontalPodAutoscalerPatch {
	return ogcapi.Spec.HorizontalPodAutoscalerPatch
}

// OGCAPIList contains a list of OGCAPI
// +kubebuilder:object:root=true
type OGCAPIList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OGCAPI `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OGCAPI{}, &OGCAPIList{})
}
