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

package controller

import (
	"context"
	"fmt"
	"strings"

	"github.com/PDOK/ogcapi-operator/internal/integrations/slack"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	traefikiov1alpha1 "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	pdoknlv1alpha1 "github.com/PDOK/ogcapi-operator/api/v1alpha1"
	smoothoperatorstatus "github.com/pdok/smooth-operator/pkg/status"

	smoothoperatorutil "github.com/pdok/smooth-operator/pkg/util"
)

const (
	controllerName      = "ogcapi-controller"
	appLabelKey         = "pdok.nl/app"
	gokoalaName         = "gokoala"
	configName          = "config"
	configFileName      = "config.yaml"
	gpkgCacheName       = "gpkg-cache"
	mainPortName        = "main"
	mainPortNr          = 8080
	debugPortName       = "debug"
	debugPortNr         = 9001
	debugPortEnvVar     = "DEBUG_PORT"
	configFileEnvVar    = "CONFIG_FILE"
	shutdownDelayEnvVar = "SHUTDOWN_DELAY"
	stripPrefixName     = "strip-prefix"
	headersName         = "cors-headers"
	srvDir              = "/srv"
	priorityAnnotation  = "priority.version-checker.io"

	volumeMountPath = "/data"
)

var (
	finalizerName = controllerName + "." + pdoknlv1alpha1.GroupVersion.Group + "/finalizer"
)

// OGCAPIReconciler reconciles a OGCAPI object
type OGCAPIReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	GokoalaImage string
	CSP          string
	Slack        slack.Sender
}

// +kubebuilder:rbac:groups=pdok.nl,resources=ogcapis,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=pdok.nl,resources=ogcapis/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=pdok.nl,resources=ogcapis/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps;services,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=traefik.io,resources=ingressroutes;middlewares,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=create;update;delete;list;watch
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets/status,verbs=get;update
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *OGCAPIReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	lgr := log.FromContext(ctx)

	ogcAPI := &pdoknlv1alpha1.OGCAPI{}
	err = r.Get(ctx, req.NamespacedName, ogcAPI)
	if err != nil {
		if apierrors.IsNotFound(err) {
			lgr.Info("OGCAPI resource not found", "name", req.NamespacedName)
		} else {
			errMsg := "unable to fetch OGCAPI resource"
			r.Slack.Send(ctx, errMsg)
			lgr.Error(err, errMsg, "error", err)
		}
		return result, client.IgnoreNotFound(err)
	}
	fullName := getObjectFullName(r.Client, ogcAPI)

	shouldContinue, err := finalizeIfNecessary(ctx, r.Client, ogcAPI, finalizerName, func() error {
		lgr.Info("deleting resources", "name", fullName)
		return r.deleteAll(ctx, ogcAPI)
	})
	if !shouldContinue || err != nil {
		return result, err
	}

	operationResults, err := r.createOrUpdate(ctx, ogcAPI)
	if err != nil {
		r.logAndUpdateStatusError(ctx, ogcAPI, err)
		return result, err
	}
	smoothoperatorstatus.LogAndUpdateStatusFinished(ctx, r.Client, ogcAPI, operationResults)

	return result, err
}

func (r *OGCAPIReconciler) logAndUpdateStatusError(ctx context.Context, ogcAPI *pdoknlv1alpha1.OGCAPI, err error) {
	r.Slack.Send(ctx, err.Error())
	smoothoperatorstatus.LogAndUpdateStatusError(ctx, r.Client, ogcAPI, err)
}

func (r *OGCAPIReconciler) createOrUpdate(ctx context.Context, ogcAPI *pdoknlv1alpha1.OGCAPI) (operationResults map[string]controllerutil.OperationResult, err error) {
	operationResults = make(map[string]controllerutil.OperationResult, 7)
	c := r.Client

	configMap := getBareConfigMap(ogcAPI)
	// mutate (also) before to get the hash suffix in the name
	if err = r.mutateConfigMap(ogcAPI, configMap); err != nil {
		return operationResults, err
	}
	operationResults[getObjectFullName(r.Client, configMap)], err = controllerutil.CreateOrUpdate(ctx, r.Client, configMap, func() error {
		return r.mutateConfigMap(ogcAPI, configMap)
	})
	if err != nil {
		return operationResults, fmt.Errorf("unable to create/update resource %s: %w", getObjectFullName(c, configMap), err)
	}

	deployment := getBareDeployment(ogcAPI)
	operationResults[getObjectFullName(r.Client, deployment)], err = controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		return r.mutateDeployment(ogcAPI, deployment, configMap.GetName())
	})
	if err != nil && !strings.Contains(err.Error(), "the object has been modified; please apply your changes to the latest version and try again") {
		return operationResults, fmt.Errorf("unable to create/update resource %s: %w", getObjectFullName(c, deployment), err)
	}

	service := getBareService(ogcAPI)
	operationResults[getObjectFullName(r.Client, service)], err = controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		return r.mutateService(ogcAPI, service)
	})
	if err != nil {
		return operationResults, fmt.Errorf("unable to create/update resource %s: %w", getObjectFullName(c, service), err)
	}

	stripPrefixMiddleware := getBareStripPrefixMiddleware(ogcAPI)
	operationResults[getObjectFullName(r.Client, stripPrefixMiddleware)], err = controllerutil.CreateOrUpdate(ctx, r.Client, stripPrefixMiddleware, func() error {
		return r.mutateStripPrefixMiddleware(ogcAPI, stripPrefixMiddleware)
	})
	if err != nil {
		return operationResults, fmt.Errorf("could not create or update resource %s: %w", getObjectFullName(c, stripPrefixMiddleware), err)
	}

	headersMiddleware := getBareHeadersMiddleware(ogcAPI)
	operationResults[getObjectFullName(r.Client, headersMiddleware)], err = controllerutil.CreateOrUpdate(ctx, r.Client, headersMiddleware, func() error {
		return r.mutateHeadersMiddleware(ogcAPI, headersMiddleware, r.CSP)
	})
	if err != nil {
		return operationResults, fmt.Errorf("could not create or update resource %s: %w", getObjectFullName(c, headersMiddleware), err)
	}

	ingressRoute := getBareIngressRoute(ogcAPI)
	operationResults[getObjectFullName(r.Client, ingressRoute)], err = controllerutil.CreateOrUpdate(ctx, r.Client, ingressRoute, func() error {
		return r.mutateIngressRoute(ogcAPI, ingressRoute)
	})
	if err != nil {
		return operationResults, fmt.Errorf("unable to create/update resource %s: %w", getObjectFullName(c, ingressRoute), err)
	}

	hpa := getBareHorizontalPodAutoscaler(ogcAPI)
	operationResults[getObjectFullName(r.Client, hpa)], err = controllerutil.CreateOrUpdate(ctx, r.Client, hpa, func() error {
		return r.mutateHorizontalPodAutoscaler(ogcAPI, hpa)
	})
	if err != nil && !strings.Contains(err.Error(), "the object has been modified; please apply your changes to the latest version and try again") {
		return operationResults, fmt.Errorf("unable to create/update resource %s: %w", getObjectFullName(c, deployment), err)
	}

	podDisruptionBudget := getBarePodDisruptionBudget(ogcAPI)
	operationResults[smoothoperatorutil.GetObjectFullName(r.Client, podDisruptionBudget)], err = controllerutil.CreateOrUpdate(ctx, r.Client, podDisruptionBudget, func() error {
		return r.mutatePodDisruptionBudget(ogcAPI, podDisruptionBudget)
	})
	if err != nil {
		return operationResults, fmt.Errorf("unable to create/update resource %s: %w", smoothoperatorutil.GetObjectFullName(c, podDisruptionBudget), err)
	}

	return operationResults, nil
}

func (r *OGCAPIReconciler) deleteAll(ctx context.Context, ogcAPI *pdoknlv1alpha1.OGCAPI) (err error) {
	configMap := getBareConfigMap(ogcAPI)
	// mutate (also) before to get the hash suffix in the name
	if err = r.mutateConfigMap(ogcAPI, configMap); err != nil {
		return
	}
	return deleteObjects(ctx, r.Client, []client.Object{
		configMap,
		getBareDeployment(ogcAPI),
		getBareService(ogcAPI),
		getBareStripPrefixMiddleware(ogcAPI),
		getBareHeadersMiddleware(ogcAPI),
		getBareIngressRoute(ogcAPI),
		getBareHorizontalPodAutoscaler(ogcAPI),
	})
}

// SetupWithManager sets up the controller with the Manager.
func (r *OGCAPIReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&pdoknlv1alpha1.OGCAPI{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&corev1.ConfigMap{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&appsv1.Deployment{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&corev1.Service{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&traefikiov1alpha1.Middleware{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&traefikiov1alpha1.IngressRoute{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&appsv1.ReplicaSet{}, smoothoperatorstatus.GetReplicaSetEventHandlerForObj(mgr, "OGCAPI")).
		Complete(r)
}
