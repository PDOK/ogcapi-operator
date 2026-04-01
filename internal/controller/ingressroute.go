package controller

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	v1alpha2 "github.com/PDOK/ogcapi-operator/api/v1alpha1"
	"github.com/pdok/smooth-operator/model"
	uptimeutils "github.com/pdok/smooth-operator/pkg/uptime-utils"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

func getBareIngressRoute(ogcAPI v1.Object) *v1alpha1.IngressRoute {
	return &v1alpha1.IngressRoute{
		ObjectMeta: v1.ObjectMeta{
			Name:      ogcAPI.GetName(),
			Namespace: ogcAPI.GetNamespace(),
		},
	}
}

func (r *OGCAPIReconciler) mutateIngressRoute(ogcAPI *v1alpha2.OGCAPI, ingressRoute *v1alpha1.IngressRoute) error {

	name := ingressRoute.GetName()
	labels := getLabels(ogcAPI)
	if err := setImmutableLabels(r.Client, ingressRoute, labels); err != nil {
		return err
	}

	ingressRoute.Annotations = uptimeutils.GetUptimeAnnotations(
		ogcAPI.Annotations,
		getBareService(ogcAPI).GetName()+"-ogcapi",
		fmt.Sprintf("%s %s OGC API", ogcAPI.Spec.Service.Title, ogcAPI.Spec.Service.Version),
		ogcAPI.Spec.Service.BaseURL.String()+"/health",
		ogcAPI.Labels,
	)

	ingressRoute.Spec.Routes = []v1alpha1.Route{}

	// Collect all ingressRouteURLs (aliases)
	ingressRouteURLs := ogcAPI.Spec.IngressRouteURLs
	if len(ingressRouteURLs) == 0 {
		ingressRouteURLs = model.IngressRouteURLs{{URL: model.URL{URL: ogcAPI.Spec.Service.BaseURL.URL}}}
	}

	for _, ingressRouteURL := range ingressRouteURLs {
		matchRule := getMatchRuleForURL(*ingressRouteURL.URL.URL, true, true)
		ingressRoute.Spec.Routes = append(
			ingressRoute.Spec.Routes,
			v1alpha1.Route{
				Kind:   "Rule",
				Match:  matchRule,
				Syntax: "v3",
				Services: []v1alpha1.Service{
					{
						LoadBalancerSpec: v1alpha1.LoadBalancerSpec{
							Name: getBareService(ogcAPI).GetName(),
							Kind: "Service",
							Port: intstr.FromString(mainPortName),
						},
					},
				},
				Middlewares: []v1alpha1.MiddlewareRef{
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
	return controllerruntime.SetControllerReference(ogcAPI, ingressRoute, r.Scheme)
}

func getMatchRuleForURL(url url.URL, includeLocalhost bool, matchUnderscoreVersions bool) string {
	var hostMatch string
	if includeLocalhost {
		hostMatch = fmt.Sprintf("(Host(`localhost`) || Host(`%s`))", url.Hostname())
	} else {
		hostMatch = fmt.Sprintf("Host(`%s`)", url.Hostname())
	}

	path := url.EscapedPath()
	trailingSlash := strings.HasSuffix(path, "/")
	path = strings.Trim(path, "/")
	if path == "" {
		return hostMatch
	}

	var pathRegexp string
	if matchUnderscoreVersions {
		pathRegexp = createRegexpForUnderscoreVersions(path)
	} else {
		pathRegexp = regexp.QuoteMeta(path)
	}

	trailingRegexp := "(/|$)" // to prevent matching too much after the last segment
	if trailingSlash {
		trailingRegexp = "/"
	}

	pathMatch := fmt.Sprintf("PathRegexp(`^/%s%s`)", pathRegexp, trailingRegexp)
	matchRule := fmt.Sprintf("%s && %s", hostMatch, pathMatch)
	return matchRule
}
