/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	pdoknlv1alpha1 "github.com/PDOK/ogcapi-operator/api/v1alpha1"
)

// nolint:unused
// log is for logging in this package.
var ogcapilog = logf.Log.WithName("ogcapi-resource")

// SetupOGCAPIWebhookWithManager registers the webhook for OGCAPI in the manager.
func SetupOGCAPIWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&pdoknlv1alpha1.OGCAPI{}).
		WithValidator(&OGCAPICustomValidator{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-pdok-nl-v1alpha1-ogcapi,mutating=false,failurePolicy=fail,sideEffects=None,groups=pdok.nl,resources=ogcapis,verbs=create;update,versions=v1alpha1,name=vogcapi-v1alpha1.kb.io,admissionReviewVersions=v1

// OGCAPICustomValidator struct is responsible for validating the OGCAPI resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type OGCAPICustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &OGCAPICustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type OGCAPI.
func (v *OGCAPICustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	ogcapi, ok := obj.(*pdoknlv1alpha1.OGCAPI)
	if !ok {
		return nil, fmt.Errorf("expected a OGCAPI object but got %T", obj)
	}
	ogcapilog.Info("Validation for OGCAPI upon creation", "name", ogcapi.GetName())

	// TODO(user): fill in your validation logic upon object creation.

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type OGCAPI.
func (v *OGCAPICustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	ogcapi, ok := newObj.(*pdoknlv1alpha1.OGCAPI)
	if !ok {
		return nil, fmt.Errorf("expected a OGCAPI object for the newObj but got %T", newObj)
	}
	ogcapilog.Info("Validation for OGCAPI upon update", "name", ogcapi.GetName())

	// TODO(user): fill in your validation logic upon object update.

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type OGCAPI.
func (v *OGCAPICustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	ogcapi, ok := obj.(*pdoknlv1alpha1.OGCAPI)
	if !ok {
		return nil, fmt.Errorf("expected a OGCAPI object but got %T", obj)
	}
	ogcapilog.Info("Validation for OGCAPI upon deletion", "name", ogcapi.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}
