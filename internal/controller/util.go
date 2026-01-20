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
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"regexp"
	"strings"

	"github.com/pdok/smooth-operator/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/intstr"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/hasher"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"

	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

func deleteObjects(ctx context.Context, c client.Client, objects []client.Object) (err error) {
	for _, obj := range objects {
		fullName := getObjectFullName(c, obj)
		err = client.IgnoreNotFound(c.Delete(ctx, obj))
		if err != nil {
			return fmt.Errorf("unable to delete resource %s: %w", fullName, err)
		}
	}
	return
}

func finalizeIfNecessary(ctx context.Context, c client.Client, obj client.Object, finalizerName string, finalizer func() error) (shouldContinue bool, err error) {
	// not under deletion, ensure finalizer annotation
	if obj.GetDeletionTimestamp().IsZero() {
		if !controllerutil.ContainsFinalizer(obj, finalizerName) {
			controllerutil.AddFinalizer(obj, finalizerName)
			err = c.Update(ctx, obj)
			return false, err
		}
		return true, nil
	}

	// under deletion but not our finalizer annotation, do nothing
	if !controllerutil.ContainsFinalizer(obj, finalizerName) {
		return false, nil
	}

	// run finalizer and remove annotation
	if err = finalizer(); err != nil {
		return false, err
	}
	controllerutil.RemoveFinalizer(obj, finalizerName)
	err = c.Update(ctx, obj)
	return false, err
}

func getLabels(ogcAPI metav1.Object) map[string]string {
	labels := cloneOrEmptyMap(ogcAPI.GetLabels())
	labels[appLabelKey] = gokoalaName
	return labels
}

func setImmutableLabels(c client.Client, obj client.Object, labels map[string]string) error {
	objLabels := obj.GetLabels()
	if obj.GetResourceVersion() != "" || len(objLabels) > 0 {
		if !equality.Semantic.DeepEqual(labels, objLabels) {
			return fmt.Errorf("labels on %s are immutable", getObjectFullName(c, obj))
		}
	}
	obj.SetLabels(labels)
	return nil
}

func strategicMergePatch[T, P any](obj *T, patch *P) (*T, error) {
	objJSON, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal the object: %w", err)
	}

	patchJSON, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal the patch: %w", err)
	}

	newJSON, err := strategicpatch.StrategicMergePatch(objJSON, patchJSON, obj) // TODO obj can be used as dataStruct?
	if err != nil {
		return nil, fmt.Errorf("error while strategic merge patching: %w", err)
	}

	var newObj T
	err = json.Unmarshal(newJSON, &newObj)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling after strategic merge patching: %w", err)
	}
	return &newObj, nil
}

func getMatchRuleForUrl(url url.URL, includeLocalhost bool, matchUnderscoreVersions bool) string {
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

func getStripPrefixesRegexps(baseUrl url.URL, ingressRouteUrls model.IngressRouteURLs, matchUnderscoreVersions bool) []string {
	result := make([]string, 0)
	var inputs []url.URL
	if ingressRouteUrls != nil && len(ingressRouteUrls) > 0 {
		inputs = make([]url.URL, 0)
		for _, route := range ingressRouteUrls {
			if route.URL.URL != nil {
				inputs = append(inputs, *route.URL.URL)
			}
		}
	} else {
		inputs = []url.URL{baseUrl}
	}

	for _, ingressUrl := range inputs {
		path := ingressUrl.EscapedPath()
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

func addHashSuffix(obj client.Object) error {
	orgName := obj.GetName()
	bareName, existingHash := splitHashSuffix(obj.GetName())
	obj.SetName(bareName)
	hash, err := kustomizeHash(obj)
	if err != nil {
		obj.SetName(orgName)
		return err
	}
	if existingHash != "" {
		obj.SetName(orgName)
		if existingHash != hash {
			return errors.New(orgName + " is already hashed with a different hash than " + hash)
		}
		return nil
	}
	obj.SetName(bareName + "-" + hash)
	return nil
}

// pattern for a name with a hash suffix. the character set is deduced from hasher.encode
var hashSuffixedRegex = regexp.MustCompile(`^(.+?)(?:-([gh2k4567890mbcdtf]{10}))?$`)

func splitHashSuffix(in string) (name, hash string) {
	m := hashSuffixedRegex.FindStringSubmatch(in)
	if len(m) >= 2 {
		return m[1], m[2]
	}
	return in, ""
}

// kustomizeHash aims to calculate a hash for an object the same way kustomize does.
// please make sure your object has its Kind set.
func kustomizeHash(obj client.Object) (hash string, err error) {
	objJSON, err := json.Marshal(obj)
	if err != nil {
		return
	}
	objKYaml, err := kyaml.ConvertJSONToYamlNode(string(objJSON))
	if err != nil {
		return
	}
	kustomizeHasher := hasher.Hasher{}
	return kustomizeHasher.Hash(objKYaml)
}

func ensureSetGVK(c client.Client, src client.Object, obj schema.ObjectKind) error {
	gvk, err := c.GroupVersionKindFor(src)
	if err != nil {
		return err
	}
	obj.SetGroupVersionKind(gvk)
	return nil
}

func getObjectFullName(c client.Client, obj client.Object) string {
	gvk, _ := c.GroupVersionKindFor(obj)
	key := client.ObjectKeyFromObject(obj)
	return gvk.Group + "/" + gvk.Version + "/" + gvk.Kind + "/" + key.String()
}

func boolPtr(b bool) *bool {
	return &b
}

func int32Ptr(i int32) *int32 {
	return &i
}

func intOrStrIntPtr(i int32) *intstr.IntOrString {
	v := intstr.FromInt32(i)
	return &v
}

func cloneOrEmptyMap[K comparable, V any](m map[K]V) map[K]V {
	if m == nil {
		return map[K]V{}
	}
	return maps.Clone(m)
}

type ErrorClient struct {
	client.Client
	err error
}

func (e *ErrorClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return e.err
}
