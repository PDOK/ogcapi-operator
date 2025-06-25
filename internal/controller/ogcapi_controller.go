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
	"crypto/sha1" //nolint:gosec  // sha1 is only used for ID generation here, not crypto
	"fmt"
	"strconv"
	"time"

	"github.com/PDOK/ogcapi-operator/internal/integrations/slack"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"

	yaml "sigs.k8s.io/yaml/goyaml.v3"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	traefikdynamic "github.com/traefik/traefik/v3/pkg/config/dynamic"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"

	traefikiov1alpha1 "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	pdoknlv1alpha1 "github.com/PDOK/ogcapi-operator/api/v1alpha1"
	smoothoperatormodel "github.com/pdok/smooth-operator/model"
)

const (
	reconciledConditionType         = "Reconciled"
	reconciledConditionReasonSucces = "Succes"
	reconciledConditionReasonError  = "Error"
)

const (
	controllerName      = "ogcapi-controller"
	appLabelKey         = "app"
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

//+kubebuilder:rbac:groups=pdok.nl,resources=ogcapis,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=pdok.nl,resources=ogcapis/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=pdok.nl,resources=ogcapis/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;delete
//+kubebuilder:rbac:groups=core,resources=configmaps;services,verbs=get;list;watch;create;update;delete
//+kubebuilder:rbac:groups=traefik.io,resources=ingressroutes;middlewares,verbs=get;list;watch;create;update;delete
//+kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *OGCAPIReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	lgr := log.FromContext(ctx)

	ogcAPI := &pdoknlv1alpha1.OGCAPI{}
	err = r.Client.Get(ctx, req.NamespacedName, ogcAPI)
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
		return r.deleteAllForOGCAPI(ctx, ogcAPI)
	})
	if !shouldContinue || err != nil {
		return result, err
	}

	operationResults, err := r.createOrUpdateAllForOGCAPI(ctx, ogcAPI)
	if err != nil {
		r.logAndUpdateStatusError(ctx, ogcAPI, err)
		return result, err
	}
	r.logAndUpdateStatusFinished(ctx, ogcAPI, operationResults)

	return result, err
}

func (r *OGCAPIReconciler) logAndUpdateStatusError(ctx context.Context, ogcAPI *pdoknlv1alpha1.OGCAPI, err error) {
	r.Slack.Send(ctx, err.Error())
	r.updateStatus(ctx, ogcAPI, []metav1.Condition{{
		Type:               reconciledConditionType,
		Status:             metav1.ConditionFalse,
		Reason:             reconciledConditionReasonError,
		Message:            err.Error(),
		ObservedGeneration: ogcAPI.Generation,
		LastTransitionTime: metav1.NewTime(time.Now()),
	}}, nil)
}

func (r *OGCAPIReconciler) createOrUpdateAllForOGCAPI(ctx context.Context, ogcAPI *pdoknlv1alpha1.OGCAPI) (operationResults map[string]controllerutil.OperationResult, err error) {
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
	if err != nil {
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
	if err != nil {
		return operationResults, fmt.Errorf("unable to create/update resource %s: %w", getObjectFullName(c, hpa), err)
	}

	return operationResults, nil
}

func (r *OGCAPIReconciler) deleteAllForOGCAPI(ctx context.Context, ogcAPI *pdoknlv1alpha1.OGCAPI) (err error) {
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

func getBareDeployment(ogcAPI metav1.Object) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: ogcAPI.GetName() + "-" + gokoalaName,
			// name might become too long. not handling here. will just fail on apply.
			Namespace: ogcAPI.GetNamespace(),
		},
	}
}

//nolint:funlen
func (r *OGCAPIReconciler) mutateDeployment(ogcAPI *pdoknlv1alpha1.OGCAPI, deployment *appsv1.Deployment, configMapName string) error {
	labels := cloneOrEmptyMap(ogcAPI.GetLabels())
	labels[appLabelKey] = gokoalaName
	if err := setImmutableLabels(r.Client, deployment, labels); err != nil {
		return err
	}

	podTemplateAnnotations := cloneOrEmptyMap(deployment.Spec.Template.GetAnnotations())
	podTemplateAnnotations[priorityAnnotation+"/"+gokoalaName] = "4"

	matchLabels := cloneOrEmptyMap(labels)
	deployment.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: matchLabels,
	}

	deployment.Spec.MinReadySeconds = 0
	deployment.Spec.ProgressDeadlineSeconds = int32Ptr(600)
	deployment.Spec.Strategy = appsv1.DeploymentStrategy{
		Type: appsv1.RollingUpdateDeploymentStrategyType,
		RollingUpdate: &appsv1.RollingUpdateDeployment{
			MaxUnavailable: intOrStrStrPtr("25%"),
			MaxSurge:       intOrStrStrPtr("25%"),
		},
	}
	deployment.Spec.RevisionHistoryLimit = int32Ptr(3)

	// deployment.Spec.Replicas is controlled by the HPA
	// deployment.Spec.Paused is ignored to allow a manual intervention i.c.e.

	podTemplateSpec := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      matchLabels,
			Annotations: podTemplateAnnotations,
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{Name: gokoalaName + "-" + configName, VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapName,
					},
				}}},
				{Name: gokoalaName + "-" + gpkgCacheName, VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
			},
			Containers: []corev1.Container{
				{
					Name:            gokoalaName,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Ports: []corev1.ContainerPort{
						{Name: mainPortName, ContainerPort: mainPortNr},
						{Name: debugPortName, ContainerPort: debugPortNr},
					},
					Env: []corev1.EnvVar{
						{Name: configFileEnvVar, Value: srvDir + "/" + configName + "/" + configFileName},
						{Name: debugPortEnvVar, Value: strconv.Itoa(debugPortNr)},
						{Name: shutdownDelayEnvVar, Value: strconv.Itoa(30)},
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceMemory:           resource.MustParse("1Gi"),
							corev1.ResourceEphemeralStorage: resource.MustParse("50Mi"), // TODO other sane default in case of OGC API Features
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("500m"),
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: gokoalaName + "-" + configName, MountPath: srvDir + "/" + configName},
						{Name: gokoalaName + "-" + gpkgCacheName, MountPath: srvDir + "/" + gpkgCacheName},
					},
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path:   "/health",
								Port:   intstr.FromInt32(mainPortNr),
								Scheme: corev1.URISchemeHTTP,
							},
						},
						InitialDelaySeconds: 60,
						TimeoutSeconds:      5,
						PeriodSeconds:       10,
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path:   "/health",
								Port:   intstr.FromInt32(mainPortNr),
								Scheme: corev1.URISchemeHTTP,
							},
						},
						InitialDelaySeconds: 60,
						TimeoutSeconds:      5,
						PeriodSeconds:       10,
					},
				},
			},
		},
	}

	if ogcAPI.Spec.PodSpecPatch != nil {
		patchedPod, err := strategicMergePatch(&podTemplateSpec.Spec, &ogcAPI.Spec.PodSpecPatch)
		if err != nil {
			return err
		}
		podTemplateSpec.Spec = *patchedPod
	}
	podTemplateSpec.Spec.Containers[0].Image = r.GokoalaImage
	deployment.Spec.Template = podTemplateSpec

	if err := ensureSetGVK(r.Client, deployment, deployment); err != nil {
		return err
	}
	return ctrl.SetControllerReference(ogcAPI, deployment, r.Scheme)
}

// getBareConfigMap sets the base name for the configmap containing the config for the gokoala Deployment.
// A hash suffix is/should be added to the actual full ConfigMap later.
func getBareConfigMap(ogcAPI metav1.Object) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getBareDeployment(ogcAPI).GetName(),
			Namespace: ogcAPI.GetNamespace(),
		},
	}
}

func (r *OGCAPIReconciler) mutateConfigMap(ogcAPI *pdoknlv1alpha1.OGCAPI, configMap *corev1.ConfigMap) error {
	labels := cloneOrEmptyMap(ogcAPI.GetLabels())
	labels[appLabelKey] = gokoalaName
	if err := setImmutableLabels(r.Client, configMap, labels); err != nil {
		return err
	}

	configYaml, err := yaml.Marshal(ogcAPI.Spec.Service)
	if err != nil {
		return err
	}
	configMap.Immutable = boolPtr(true)
	configMap.Data = map[string]string{configFileName: string(configYaml)}

	if err := ensureSetGVK(r.Client, configMap, configMap); err != nil {
		return err
	}
	if err := ctrl.SetControllerReference(ogcAPI, configMap, r.Scheme); err != nil {
		return err
	}
	return addHashSuffix(configMap)
}

func getBareService(ogcAPI metav1.Object) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ogcAPI.GetName(),
			Namespace: ogcAPI.GetNamespace(),
		},
	}
}

func (r *OGCAPIReconciler) mutateService(ogcAPI *pdoknlv1alpha1.OGCAPI, service *corev1.Service) error {
	labels := cloneOrEmptyMap(ogcAPI.GetLabels())
	selector := cloneOrEmptyMap(ogcAPI.GetLabels())
	selector[appLabelKey] = gokoalaName
	if err := setImmutableLabels(r.Client, service, labels); err != nil {
		return err
	}

	internalTrafficPolicy := corev1.ServiceInternalTrafficPolicyCluster
	service.Spec = corev1.ServiceSpec{
		Type:                  corev1.ServiceTypeClusterIP,
		ClusterIP:             service.Spec.ClusterIP,
		ClusterIPs:            service.Spec.ClusterIPs,
		IPFamilyPolicy:        service.Spec.IPFamilyPolicy,
		IPFamilies:            service.Spec.IPFamilies,
		SessionAffinity:       corev1.ServiceAffinityNone,
		InternalTrafficPolicy: &internalTrafficPolicy,
		Ports: []corev1.ServicePort{
			{
				Name:       mainPortName,
				Protocol:   corev1.ProtocolTCP,
				Port:       mainPortNr,
				TargetPort: intstr.FromInt32(mainPortNr),
			},
		},
		Selector: selector,
	}
	if err := ensureSetGVK(r.Client, service, service); err != nil {
		return err
	}
	return ctrl.SetControllerReference(ogcAPI, service, r.Scheme)
}

func getBareIngressRoute(ogcAPI metav1.Object) *traefikiov1alpha1.IngressRoute {
	return &traefikiov1alpha1.IngressRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ogcAPI.GetName(),
			Namespace: ogcAPI.GetNamespace(),
		},
	}
}

func (r *OGCAPIReconciler) mutateIngressRoute(ogcAPI *pdoknlv1alpha1.OGCAPI, ingressRoute *traefikiov1alpha1.IngressRoute) error {
	uptimeURL := ogcAPI.Spec.Service.BaseURL.String() + "/health"
	name := ingressRoute.GetName()
	labels := cloneOrEmptyMap(ogcAPI.GetLabels())
	if err := setImmutableLabels(r.Client, ingressRoute, labels); err != nil {
		return err
	}
	ingressRoute.Annotations = map[string]string{
		"uptime.pdok.nl/id":   fmt.Sprintf("%x", sha1.Sum([]byte(getBareService(ogcAPI).GetName()+"-ogcapi"))), //nolint:gosec  // sha1 is only used for ID generation here, not crypto
		"uptime.pdok.nl/name": fmt.Sprintf("%s %s OGC API", ogcAPI.Spec.Service.Title, ogcAPI.Spec.Service.Version),
		"uptime.pdok.nl/url":  uptimeURL,
		"uptime.pdok.nl/tags": "public-stats,ogcapi",
	}

	ingressRoute.Spec.Routes = []traefikiov1alpha1.Route{}

	// Collect all ingressRouteURLs (aliases)
	ingressRouteURLs := ogcAPI.Spec.IngressRouteURLs
	if len(ingressRouteURLs) == 0 {
		ingressRouteURLs = smoothoperatormodel.IngressRouteURLs{{URL: smoothoperatormodel.URL{URL: ogcAPI.Spec.Service.BaseURL.URL}}}
	}

	for _, ingressRouteURL := range ingressRouteURLs {
		matchRule, _ := createIngressRuleAndStripPrefixForBaseURL(*ingressRouteURL.URL.URL, true, true)
		ingressRoute.Spec.Routes = append(
			ingressRoute.Spec.Routes,
			traefikiov1alpha1.Route{
				Kind:   "Rule",
				Match:  matchRule,
				Syntax: "v3",
				Services: []traefikiov1alpha1.Service{
					{
						LoadBalancerSpec: traefikiov1alpha1.LoadBalancerSpec{
							Name: getBareService(ogcAPI).GetName(),
							Kind: "Service",
							Port: intstr.FromString(mainPortName),
						},
					},
				},
				Middlewares: []traefikiov1alpha1.MiddlewareRef{
					{
						Name:      name + "-" + stripPrefixName,
						Namespace: ogcAPI.GetNamespace(),
					},
					{
						Name:      name + "-" + headersName,
						Namespace: ogcAPI.GetNamespace(),
					},
				},
			},
		)
	}
	if err := ensureSetGVK(r.Client, ingressRoute, ingressRoute); err != nil {
		return err
	}
	return ctrl.SetControllerReference(ogcAPI, ingressRoute, r.Scheme)
}

func getBareStripPrefixMiddleware(ogcAPI metav1.Object) *traefikiov1alpha1.Middleware {
	return &traefikiov1alpha1.Middleware{
		ObjectMeta: metav1.ObjectMeta{
			Name: ogcAPI.GetName() + "-" + stripPrefixName,
			// name might become too long. not handling here. will just fail on apply.
			Namespace: ogcAPI.GetNamespace(),
		},
	}
}

func (r *OGCAPIReconciler) mutateStripPrefixMiddleware(ogcAPI *pdoknlv1alpha1.OGCAPI, middleware *traefikiov1alpha1.Middleware) error {
	labels := cloneOrEmptyMap(ogcAPI.GetLabels())
	if err := setImmutableLabels(r.Client, middleware, labels); err != nil {
		return err
	}
	_, stripPrefixRegex := createIngressRuleAndStripPrefixForBaseURL(*ogcAPI.Spec.Service.BaseURL.URL, true, true)
	middleware.Spec = traefikiov1alpha1.MiddlewareSpec{
		StripPrefixRegex: &traefikdynamic.StripPrefixRegex{
			Regex: []string{
				stripPrefixRegex,
			},
		},
	}
	if err := ensureSetGVK(r.Client, middleware, middleware); err != nil {
		return err
	}
	return ctrl.SetControllerReference(ogcAPI, middleware, r.Scheme)
}

func getBareHeadersMiddleware(obj metav1.Object) *traefikiov1alpha1.Middleware {
	return &traefikiov1alpha1.Middleware{
		ObjectMeta: metav1.ObjectMeta{
			Name: obj.GetName() + "-" + headersName,
			// name might become too long. not handling here. will just fail on apply.
			Namespace: obj.GetNamespace(),
		},
	}
}

func (r *OGCAPIReconciler) mutateHeadersMiddleware(obj metav1.Object, middleware *traefikiov1alpha1.Middleware, csp string) error {
	labels := cloneOrEmptyMap(obj.GetLabels())
	if err := setImmutableLabels(r.Client, middleware, labels); err != nil {
		return err
	}
	middleware.Spec = traefikiov1alpha1.MiddlewareSpec{
		Headers: &traefikdynamic.Headers{
			// CORS
			AccessControlAllowHeaders: []string{
				"X-Requested-With",
			},
			AccessControlAllowMethods: []string{
				"GET",
				"HEAD",
				"OPTIONS",
			},
			AccessControlAllowOriginList: []string{
				"*",
			},
			AccessControlExposeHeaders: []string{
				"Content-Crs",
				"Link",
			},
			AccessControlMaxAge: 86400,
			// CSP
			ContentSecurityPolicy: csp,
			// Frame-Options
			FrameDeny: true,
			// Other headers
			CustomResponseHeaders: map[string]string{
				"Cache-Control": "public, max-age=3600, no-transform",
				"Vary":          "Cookie, Accept, Accept-Encoding, Accept-Language",
			},
		},
	}
	if err := ensureSetGVK(r.Client, middleware, middleware); err != nil {
		return err
	}
	return ctrl.SetControllerReference(obj, middleware, r.Scheme)
}

func getBareHorizontalPodAutoscaler(ogcAPI metav1.Object) *autoscalingv2.HorizontalPodAutoscaler {
	return &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getBareDeployment(ogcAPI).GetName(),
			Namespace: ogcAPI.GetNamespace(),
		},
	}
}

func (r *OGCAPIReconciler) mutateHorizontalPodAutoscaler(ogcAPI metav1.Object, hpa *autoscalingv2.HorizontalPodAutoscaler) error {
	labels := cloneOrEmptyMap(ogcAPI.GetLabels())
	if err := setImmutableLabels(r.Client, hpa, labels); err != nil {
		return err
	}
	hpa.Spec = autoscalingv2.HorizontalPodAutoscalerSpec{
		ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       getBareDeployment(ogcAPI).GetName(),
		},
		MinReplicas: int32Ptr(2),
		MaxReplicas: 4,
		Metrics: []autoscalingv2.MetricSpec{
			{
				Type: autoscalingv2.ResourceMetricSourceType,
				Resource: &autoscalingv2.ResourceMetricSource{
					Name: corev1.ResourceCPU,
					Target: autoscalingv2.MetricTarget{
						Type:               autoscalingv2.UtilizationMetricType,
						AverageUtilization: int32Ptr(80),
					},
				},
			},
			{
				Type: autoscalingv2.ResourceMetricSourceType,
				Resource: &autoscalingv2.ResourceMetricSource{
					Name: corev1.ResourceMemory,
					Target: autoscalingv2.MetricTarget{
						Type:               autoscalingv2.UtilizationMetricType,
						AverageUtilization: int32Ptr(75),
					},
				},
			},
		},
		Behavior: &autoscalingv2.HorizontalPodAutoscalerBehavior{
			ScaleDown: &autoscalingv2.HPAScalingRules{
				StabilizationWindowSeconds: int32Ptr(900),
				Policies: []autoscalingv2.HPAScalingPolicy{
					{
						Type:          autoscalingv2.PodsScalingPolicy,
						Value:         1,
						PeriodSeconds: 300,
					},
				},
			},
			ScaleUp: &autoscalingv2.HPAScalingRules{
				StabilizationWindowSeconds: int32Ptr(0),
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
	if err := ensureSetGVK(r.Client, hpa, hpa); err != nil {
		return err
	}
	return ctrl.SetControllerReference(ogcAPI, hpa, r.Scheme)
}

func (r *OGCAPIReconciler) logAndUpdateStatusFinished(ctx context.Context, ogcAPI *pdoknlv1alpha1.OGCAPI, operationResults map[string]controllerutil.OperationResult) {
	lgr := log.FromContext(ctx)
	lgr.Info("operation results", "results", operationResults)
	r.updateStatus(ctx, ogcAPI, []metav1.Condition{{
		Type:               reconciledConditionType,
		Status:             metav1.ConditionTrue,
		Reason:             reconciledConditionReasonSucces,
		ObservedGeneration: ogcAPI.Generation,
		LastTransitionTime: metav1.NewTime(time.Now()),
	}}, operationResults)
}

func (r *OGCAPIReconciler) updateStatus(ctx context.Context, ogcAPI *pdoknlv1alpha1.OGCAPI, conditions []metav1.Condition, operationResults map[string]controllerutil.OperationResult) {
	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(ogcAPI), ogcAPI); err != nil {
		log.FromContext(ctx).Error(err, "unable to update status")
		return
	}

	changed := false
	for _, condition := range conditions {
		if meta.SetStatusCondition(&ogcAPI.Status.Conditions, condition) {
			changed = true
		}
	}
	if !equality.Semantic.DeepEqual(ogcAPI.Status.OperationResults, operationResults) {
		ogcAPI.Status.OperationResults = operationResults
		changed = true
	}
	if !changed {
		return
	}
	if err := r.Status().Update(ctx, ogcAPI); err != nil {
		log.FromContext(ctx).Error(err, "unable to update status")
	}
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
		Complete(r)
}
