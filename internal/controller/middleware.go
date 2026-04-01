package controller

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	v1alpha2 "github.com/PDOK/ogcapi-operator/api/v1alpha1"
	"github.com/pdok/smooth-operator/model"
	"github.com/traefik/traefik/v3/pkg/config/dynamic"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

func getBareHeadersMiddleware(obj v1.Object) *v1alpha1.Middleware {
	return &v1alpha1.Middleware{
		ObjectMeta: v1.ObjectMeta{
			Name: obj.GetName() + "-" + headersName,
			// name might become too long. not handling here. will just fail on apply.
			Namespace: obj.GetNamespace(),
		},
	}
}

func (r *OGCAPIReconciler) mutateHeadersMiddleware(ogcAPI v1.Object, middleware *v1alpha1.Middleware, csp string) error {
	labels := getLabels(ogcAPI)
	if err := setImmutableLabels(r.Client, middleware, labels); err != nil {
		return err
	}
	middleware.Spec = v1alpha1.MiddlewareSpec{
		Headers: &dynamic.Headers{
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
	return controllerruntime.SetControllerReference(ogcAPI, middleware, r.Scheme)
}

func getBareStripPrefixMiddleware(ogcAPI v1.Object) *v1alpha1.Middleware {
	return &v1alpha1.Middleware{
		ObjectMeta: v1.ObjectMeta{
			Name: ogcAPI.GetName() + "-" + stripPrefixName,
			// name might become too long. not handling here. will just fail on apply.
			Namespace: ogcAPI.GetNamespace(),
		},
	}
}

func (r *OGCAPIReconciler) mutateStripPrefixMiddleware(ogcAPI *v1alpha2.OGCAPI, middleware *v1alpha1.Middleware) error {
	labels := getLabels(ogcAPI)
	if err := setImmutableLabels(r.Client, middleware, labels); err != nil {
		return err
	}
	regexes := getStripPrefixesRegexps(*ogcAPI.Spec.Service.BaseURL.URL, ogcAPI.Spec.IngressRouteURLs, true)
	middleware.Spec = v1alpha1.MiddlewareSpec{
		StripPrefixRegex: &dynamic.StripPrefixRegex{
			Regex: regexes,
		},
	}
	if err := ensureSetGVK(r.Client, middleware, middleware); err != nil {
		return err
	}
	return controllerruntime.SetControllerReference(ogcAPI, middleware, r.Scheme)
}

func getStripPrefixesRegexps(baseURL url.URL, ingressRouteUrls model.IngressRouteURLs, matchUnderscoreVersions bool) []string {
	result := make([]string, 0)
	var inputs []url.URL
	if len(ingressRouteUrls) > 0 {
		inputs = make([]url.URL, 0)
		for _, route := range ingressRouteUrls {
			if route.URL.URL != nil {
				inputs = append(inputs, *route.URL.URL)
			}
		}
	} else {
		inputs = []url.URL{baseURL}
	}

	for _, ingressURL := range inputs {
		path := ingressURL.EscapedPath()
		trailingSlash := strings.HasSuffix(path, "/")
		path = strings.Trim(path, "/")
		if path == "" {
			continue
		}

		var pathRegexp string
		if matchUnderscoreVersions {
			pathRegexp = createRegexpForUnderscoreVersions(path)
		} else {
			pathRegexp = regexp.QuoteMeta(path)
		}

		stripPrefixRegexp := fmt.Sprintf("^/%s", pathRegexp) //nolint:perfsprint
		if trailingSlash {
			stripPrefixRegexp += "/"
		}

		result = append(result, stripPrefixRegexp)
	}

	return result
}

func createRegexpForUnderscoreVersions(path string) string {
	// luckily Traefik also uses golang regular expressions syntax
	// first create a regexp that literally matches the path
	pathRegexp := regexp.QuoteMeta(path)
	// then replace any occurrences of /v1_0/ (or v2_1 or v3_6) to make the "underscore part" optional
	pathRegexp = regexp.MustCompile(`/(v\d+)(_\d+)(/|$)`).ReplaceAllString(pathRegexp, `/$1($2)?$3`)
	// then replace any occurrences of /v1/ (or v2 or v3) with a pattern for that v1 plus an optional "underscore part"
	pathRegexp = regexp.MustCompile(`/(v\d+)(/|$)`).ReplaceAllString(pathRegexp, `/$1(_\d+)?$2`)
	return pathRegexp
}
