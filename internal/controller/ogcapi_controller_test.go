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
	"net/url"

	"github.com/pkg/errors"

	"golang.org/x/text/language"

	appsv1 "k8s.io/api/apps/v1"

	traefikiov1alpha1 "github.com/traefik/traefik/v2/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gokoalaconfig "github.com/PDOK/gokoala/config"
	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo/v2" //nolint:revive // ginkgo bdd
	. "github.com/onsi/gomega"    //nolint:revive // gingko bdd
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pdoknlv1alpha1 "git.dev.cloud.kadaster.nl/pdok/ogcapi-operator/api/v1alpha1"
)

const (
	testOGCAPIName      = "test-resource"
	testOGCAPINamespace = "default"
	testServiceURL      = "https://my.test-resource.test/ogc/path"
	testImageName       = "test.test/image:test1"
	mitLicenseURL       = "https://www.tldrlegal.com/license/mit-license"
)

var minimalOGCAPI = pdoknlv1alpha1.OGCAPI{
	ObjectMeta: metav1.ObjectMeta{
		Name:      testOGCAPIName,
		Namespace: "default",
	},
	Spec: pdoknlv1alpha1.OGCAPISpec{
		Service: gokoalaconfig.Config{
			BaseURL: gokoalaconfig.URL{URL: must(url.Parse(testServiceURL))},
			License: gokoalaconfig.License{
				Name: "my license",
				URL:  gokoalaconfig.URL{URL: must(url.Parse(mitLicenseURL))},
			},
			DatasetCatalogURL: gokoalaconfig.URL{URL: must(url.Parse(testServiceURL))},
			AvailableLanguages: []gokoalaconfig.Language{
				{Tag: language.Make("nl")},
			},
		},
		PodImage: testImageName,
	},
}

var fullOGCAPI = pdoknlv1alpha1.OGCAPI{
	ObjectMeta: *minimalOGCAPI.ObjectMeta.DeepCopy(),
	Spec: pdoknlv1alpha1.OGCAPISpec{
		Service: *minimalOGCAPI.Spec.Service.DeepCopy(), // TODO fill in everything
		PodSpecPatch: &corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: "resources",
					VolumeSource: corev1.VolumeSource{Projected: &corev1.ProjectedVolumeSource{
						Sources: []corev1.VolumeProjection{
							{ConfigMap: &corev1.ConfigMapProjection{LocalObjectReference: corev1.LocalObjectReference{Name: "gokoala-resources-1"}}},
							{ConfigMap: &corev1.ConfigMapProjection{LocalObjectReference: corev1.LocalObjectReference{Name: "gokoala-resources-2"}}},
						},
					}},
				},
			},
			Containers: []corev1.Container{
				{
					Name: gokoalaName,
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "resources",
							MountPath: srvDir + "/resources",
						},
					},
				},
			},
		},
		PodImage: minimalOGCAPI.Spec.PodImage,
	},
}

var _ = Describe("OGCAPI Controller", func() {
	Context("When reconciling an OGCAPI", func() {
		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      testOGCAPIName,
			Namespace: testOGCAPINamespace,
		}
		ogcAPI := &pdoknlv1alpha1.OGCAPI{}

		BeforeEach(func() {
			By("Creating the custom resource for the Kind OGCAPI")
			err := k8sClient.Get(ctx, typeNamespacedName, ogcAPI)
			if err != nil && k8serrors.IsNotFound(err) {
				resource := fullOGCAPI.DeepCopy()
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &pdoknlv1alpha1.OGCAPI{}
			resource.Name = typeNamespacedName.Name
			resource.Namespace = typeNamespacedName.Namespace
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())

			By("Cleaning up the specific resource instance OGCAPI")
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, resource))).To(Succeed())
		})

		It("Should successfully create and delete its owned resources", func() {
			controllerReconciler := &OGCAPIReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			By("Reconciling the OGCAPI")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			By("Checking the finalizer")
			err = k8sClient.Get(ctx, typeNamespacedName, ogcAPI)
			Expect(err).NotTo(HaveOccurred())
			Expect(ogcAPI.Finalizers).To(ContainElement(finalizerName))

			By("Reconciling the OGCAPI again")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for the owned resources to be created")
			Eventually(func() error {
				configMapName, err := getGokoalaConfigMapNameFromClient(ctx, ogcAPI)
				if err != nil {
					return err
				}
				expectedBareObjects := getExpectedBareObjectsForOGCAPI(ogcAPI, configMapName)
				for _, d := range expectedBareObjects {
					err := k8sClient.Get(ctx, d.key, d.obj)
					if err != nil {
						return err
					}
				}
				return nil
			}, "10s", "1s").Should(Not(HaveOccurred()))

			By("Finding the ConfigMap name (with hash)")
			configMapName, err := getGokoalaConfigMapNameFromClient(ctx, ogcAPI)
			Expect(err).NotTo(HaveOccurred())

			By("Deleting the OGCAPI")
			Expect(k8sClient.Delete(ctx, ogcAPI)).To(Succeed())

			By("Reconciling the OGCAPI again")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for the owned resources to be deleted")
			Eventually(func() error {
				expectedBareObjects := getExpectedBareObjectsForOGCAPI(ogcAPI, configMapName)
				for _, d := range expectedBareObjects {
					err := k8sClient.Get(ctx, d.key, d.obj)
					if err == nil {
						return errors.New("expected " + getObjectFullName(k8sClient, d.obj) + " to not be found")
					}
					if !k8serrors.IsNotFound(err) {
						return err
					}
				}
				return nil
			}, "10s", "1s").Should(Not(HaveOccurred()))
		})

		It("Should successfully reconcile after a change in an owned resource", func() {
			controllerReconciler := &OGCAPIReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			By("Reconciling the OGCAPI, checking the finalizer, and reconciling again")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())
			err = k8sClient.Get(ctx, typeNamespacedName, ogcAPI)
			Expect(err).NotTo(HaveOccurred())
			Expect(ogcAPI.Finalizers).To(ContainElement(finalizerName))
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			By("Getting the original Deployment")
			deployment := getBareDeployment(ogcAPI)
			Eventually(func() bool {
				err = k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)
				return Expect(err).NotTo(HaveOccurred())
			}, "10s", "1s").Should(BeTrue())
			originalReplicas := deployment.Spec.Replicas

			By("Altering the Deployment")
			err = k8sClient.Patch(ctx, deployment, client.RawPatch(types.MergePatchType, []byte(
				`{"spec": {"replicas": 666}}`)))
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that the Deployment was altered")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)
				return Expect(err).NotTo(HaveOccurred()) &&
					Expect(deployment.Spec.Replicas).To(BeEquivalentTo(int32Ptr(666)))
			}, "10s", "1s").Should(BeTrue())

			By("Reconciling the OGCAPI again")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that the ConfigMap data was restored")
			Eventually(func() bool {
				err = k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)
				return Expect(err).NotTo(HaveOccurred()) &&
					Expect(deployment.Spec.Replicas).To(BeEquivalentTo(originalReplicas))
			}, "10s", "1s").Should(BeTrue())
		})
	})
})

func getGokoalaConfigMapNameFromClient(ctx context.Context, ogcAPI *pdoknlv1alpha1.OGCAPI) (string, error) {
	deployment := &appsv1.Deployment{}
	err := k8sClient.Get(ctx, types.NamespacedName{Namespace: testOGCAPINamespace, Name: getBareDeployment(ogcAPI).GetName()}, deployment)
	if err != nil {
		return "", err
	}
	return getGokoalaConfigMapNameFromDeployment(deployment)
}

func getGokoalaConfigMapNameFromDeployment(deployment *appsv1.Deployment) (string, error) {
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		if volume.Name == gokoalaName+"-"+configName && volume.ConfigMap != nil {
			return volume.ConfigMap.Name, nil
		}
	}
	return "", errors.New("gokoala mounted configmap not found")
}

func getExpectedBareObjectsForOGCAPI(ogcAPI *pdoknlv1alpha1.OGCAPI, configMapName string) []struct {
	obj client.Object
	key types.NamespacedName
} {
	return []struct {
		obj client.Object
		key types.NamespacedName
	}{
		{obj: &appsv1.Deployment{}, key: types.NamespacedName{Namespace: testOGCAPINamespace, Name: getBareDeployment(ogcAPI).GetName()}},
		{obj: &corev1.ConfigMap{}, key: types.NamespacedName{Namespace: testOGCAPINamespace, Name: configMapName}},
		{obj: &traefikiov1alpha1.Middleware{}, key: types.NamespacedName{Namespace: testOGCAPINamespace, Name: getBareStripPrefixMiddleware(ogcAPI).GetName()}},
		{obj: &traefikiov1alpha1.Middleware{}, key: types.NamespacedName{Namespace: testOGCAPINamespace, Name: getBareCorsHeadersMiddleware(ogcAPI).GetName()}},
		{obj: &corev1.Service{}, key: types.NamespacedName{Namespace: testOGCAPINamespace, Name: getBareService(ogcAPI).GetName()}},
		{obj: &traefikiov1alpha1.IngressRoute{}, key: types.NamespacedName{Namespace: testOGCAPINamespace, Name: getBareIngressRoute(ogcAPI).GetName()}},
		{obj: &autoscalingv2.HorizontalPodAutoscaler{}, key: types.NamespacedName{Namespace: testOGCAPINamespace, Name: getBareHorizontalPodAutoscaler(ogcAPI).GetName()}},
	}
}

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}
