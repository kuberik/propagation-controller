/*
Copyright 2023.

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

package promotion

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/registry"
	registryv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/static"
	cranev1 "github.com/google/go-containerregistry/pkg/v1/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	promotionv1alpha1 "github.com/kuberik/propagation-controller/api/promotion/v1alpha1"
)

var _ = Describe("ReleasePromotion Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		releasepromotion := &promotionv1alpha1.ReleasePromotion{}
		var registryServer *httptest.Server
		var registryEndpoint string
		var releasesRepository string
		var targetRepository string

		BeforeEach(func() {
			By("setting up the test environment")

			registry := registry.New()
			registryServer = httptest.NewServer(registry)
			registryEndpoint = strings.TrimPrefix(registryServer.URL, "http://")
			releasesRepository = fmt.Sprintf("%s/my-app/kubernetes-manifests/my-env/release", registryEndpoint)
			targetRepository = fmt.Sprintf("%s/my-app/kubernetes-manifests/my-env/deploy", registryEndpoint)

			By("creating the custom resource for the Kind ReleasePromotion")
			err := k8sClient.Get(ctx, typeNamespacedName, releasepromotion)
			if err != nil && errors.IsNotFound(err) {
				resource := &promotionv1alpha1.ReleasePromotion{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: promotionv1alpha1.ReleasePromotionSpec{
						Protocol: "oci",
						ReleasesRepository: promotionv1alpha1.Repository{
							URL: releasesRepository,
						},
						TargetRepository: promotionv1alpha1.Repository{
							URL: targetRepository,
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &promotionv1alpha1.ReleasePromotion{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ReleasePromotion")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should promote the image immediatelly when there are no gates", func() {
			By("Creating a test deployment image")
			version_0_1_0_image := pushFakeDeploymentImage(releasesRepository, "0.1.0")
			_, err := pullImage(releasesRepository, "0.1.0")
			Expect(err).ShouldNot(HaveOccurred())
			_, err = pullImage(targetRepository, "latest")
			Expect(err).Should(HaveOccurred())

			By("Reconciling the created resource")
			controllerReconciler := &ReleasePromotionReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			targetImage, err := pullImage(targetRepository, "latest")
			Expect(err).ShouldNot(HaveOccurred())
			assertEqualDigests(version_0_1_0_image, targetImage)

			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})

func pushFakeDeploymentImage(repository, version string) registryv1.Image {
	image, err := mutate.AppendLayers(empty.Image, static.NewLayer(fmt.Appendf(nil, "%s/%s", repository, version), cranev1.MediaType("fake")))
	Expect(err).ShouldNot(HaveOccurred())
	Expect(err).ShouldNot(HaveOccurred())
	pushImage(image, repository, version)
	return image
}

func pushImage(image registryv1.Image, repository, tag string) {
	imageURL := fmt.Sprintf("%s:%s", repository, tag)
	Expect(
		crane.Push(image, imageURL),
	).To(Succeed())
}

func pullImage(repository, tag string) (registryv1.Image, error) {
	imageURL := fmt.Sprintf("%s:%s", repository, tag)
	image, err := crane.Pull(imageURL)
	if err != nil {
		return nil, err
	}
	return image, nil
}

func assertEqualDigests(image1, image2 registryv1.Image) bool {
	digest1, err := image1.Digest()
	if err != nil {
		return false
	}
	digest2, err := image2.Digest()
	if err != nil {
		return false
	}
	return digest1.String() == digest2.String()
}
