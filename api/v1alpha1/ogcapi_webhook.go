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
	"fmt"

	"golang.org/x/net/context"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	smoothoperatormodel "github.com/pdok/smooth-operator/model"
	smoothoperatorvalidation "github.com/pdok/smooth-operator/pkg/validation"
)

// log is for logging in this package.
var ogcapilog = logf.Log.WithName("ogcapi-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *OGCAPI) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		WithValidator(&OGCAPICustomValidator{mgr.GetClient()}).
		For(r).
		Complete()
}

// Note: change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-pdok-nl-v1alpha1-ogcapi,mutating=false,failurePolicy=fail,sideEffects=None,groups=pdok.nl,resources=ogcapis,verbs=create;update,versions=v1alpha1,name=vogcapi.kb.io,admissionReviewVersions=v1

// OGCAPICustomValidator struct is responsible for validating the OGCAPI resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
// +kubebuilder:object:generate=false
type OGCAPICustomValidator struct {
	Client client.Client
}

var _ webhook.CustomValidator = &OGCAPICustomValidator{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (v *OGCAPICustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	ogcapi, ok := obj.(*OGCAPI)
	if !ok {
		return nil, fmt.Errorf("expected an OGCAPI object but got %T", obj)
	}
	ogcapilog.Info("validate create", "name", ogcapi.Name)

	// NOTE: Validation of the 'service' part in the OGCAPI is implicitly performed
	// by gokoalaconfig.Config.UnmarshalYAML(). No need to explicitly invoke anything.
	// Please add additional GoKoala specific validations in GoKoala's UnmarshallYAML() method.

	// Any other validations may be added below.
	if ogcapi.Spec.IngressRouteURLs != nil {
		err := smoothoperatorvalidation.ValidateIngressRouteURLsContainsBaseURL(ogcapi.Spec.IngressRouteURLs, smoothoperatormodel.URL{URL: ogcapi.Spec.Service.BaseURL.URL}, nil)

		return nil, err
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
// the 'old' Object is passed as an argument, so it can be used in the validation
func (v *OGCAPICustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldOgcapi, ok := oldObj.(*OGCAPI)
	if !ok {
		return nil, fmt.Errorf("expected an OGCAPI object but got %T", oldObj)
	}
	newOgcapi, ok := newObj.(*OGCAPI)
	if !ok {
		return nil, fmt.Errorf("expected an OGCAPI object but got %T", newObj)
	}
	ogcapilog.Info("validate update", "name", newOgcapi.Name)

	// NOTE: Validation of the 'service' part in the OGCAPI is implicitly performed
	// by gokoalaconfig.Config.UnmarshalYAML(). No need to explicitly invoke anything.
	// Please add additional GoKoala specific validations in GoKoala's UnmarshallYAML() method.
	// Any other validations may be added below.
	allErrs := field.ErrorList{}

	if newOgcapi.Spec.IngressRouteURLs == nil {
		smoothoperatorvalidation.CheckURLImmutability(
			smoothoperatormodel.URL{URL: oldOgcapi.Spec.Service.BaseURL.URL},
			smoothoperatormodel.URL{URL: newOgcapi.Spec.Service.BaseURL.URL},
			&allErrs,
			field.NewPath("spec").Child("service").Child("baseUrl"),
		)
	} else {
		if err := smoothoperatorvalidation.ValidateIngressRouteURLsContainsBaseURL(newOgcapi.Spec.IngressRouteURLs, smoothoperatormodel.URL{URL: newOgcapi.Spec.Service.BaseURL.URL}, nil); err != nil {
			allErrs = append(allErrs, err)
		}
		if err := smoothoperatorvalidation.ValidateIngressRouteURLsContainsBaseURL(newOgcapi.Spec.IngressRouteURLs, smoothoperatormodel.URL{URL: oldOgcapi.Spec.Service.BaseURL.URL}, nil); err != nil {
			allErrs = append(allErrs, err)
		}
	}

	smoothoperatorvalidation.ValidateIngressRouteURLsNotRemoved(oldOgcapi.Spec.IngressRouteURLs, newOgcapi.Spec.IngressRouteURLs, &allErrs, nil)

	smoothoperatorvalidation.ValidateLabelsOnUpdate(oldOgcapi.GetLabels(), newOgcapi.GetLabels(), &allErrs)
	if len(allErrs) > 0 {
		return nil, allErrs.ToAggregate()
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v *OGCAPICustomValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	ogcapi, ok := obj.(*OGCAPI)
	if !ok {
		return nil, fmt.Errorf("expected an OGCAPI object but got %T", obj)
	}
	ogcapilog.Info("validate delete", "name", ogcapi.Name)
	// noop
	return nil, nil
}
