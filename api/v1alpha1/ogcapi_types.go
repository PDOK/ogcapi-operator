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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OGCAPISpec defines the desired state of OGCAPI
type OGCAPISpec struct {
	Service gokoalaconfig.Config `json:"service"`
	//+kubebuilder:validation:Type=object
	//+kubebuilder:validation:Schemaless
	//+kubebuilder:pruning:PreserveUnknownFields
	// Optional strategic merge patch for the pod in the deployment. E.g. to patch the resources or add extra env vars.
	PodSpecPatch *corev1.PodSpec `json:"podSpecPatch,omitempty"`
	// Image name explicitly provided. So it can be managed with e.g. Kustomize. TODO maybe a default?
	PodImage string `json:"podImage"`
}

// OGCAPIStatus defines the observed state of OGCAPI
type OGCAPIStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// OGCAPI is the Schema for the ogcapis API
type OGCAPI struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OGCAPISpec   `json:"spec,omitempty"`
	Status OGCAPIStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OGCAPIList contains a list of OGCAPI
type OGCAPIList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OGCAPI `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OGCAPI{}, &OGCAPIList{})
}
