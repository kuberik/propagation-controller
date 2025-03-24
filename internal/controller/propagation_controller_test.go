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

package controller

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/static"
	typescrane "github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/kuberik/propagation-controller/api/v1alpha1"
	"github.com/kuberik/propagation-controller/pkg/clients"
	"github.com/kuberik/propagation-controller/pkg/repo/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// +kubebuilder:docs-gen:collapse=Imports

/*
The first step to writing a simple integration test is to actually create an instance of CronJob you can run tests against.
Note that to create a CronJob, you’ll need to create a stub CronJob struct that contains your CronJob’s specifications.

Note that when we create a stub CronJob, the CronJob also needs stubs of its required downstream objects.
Without the stubbed Job template spec and the Pod template spec below, the Kubernetes API will not be able to
create the CronJob.
*/
var _ = Describe("Propagation controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		timeout  = time.Second * 5
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("Propagating dev -> staging", func() {
		const (
			PropagationName             = "my-app"
			PropagationDevNamespace     = "dev"
			PropagationStagingNamespace = "staging"
		)
		It("Should propagate version to/from another deployment", func() {
			ctx := context.Background()

			By("Pushing manifests")
			devRev2Image := pushFakeDeploymentImage(PropagationName, "frankfurt-dev-1", "rev-2")
			stagingRev1Image := pushFakeDeploymentImage(PropagationName, "frankfurt-staging-1", "rev-1")
			stagingRev2Image := pushFakeDeploymentImage(PropagationName, "frankfurt-staging-1", "rev-2")

			config := config.Config{
				Environments: []config.Environment{{
					Name: "dev",
					Waves: []config.Wave{{
						BakeTime:    metav1.Duration{Duration: 1 * time.Hour},
						Deployments: []string{"frankfurt-dev-1"},
					}},
				}, {
					Name: "staging",
					Waves: []config.Wave{{
						BakeTime:    metav1.Duration{Duration: 4 * time.Hour},
						Deployments: []string{"frankfurt-staging-1"},
					}},
				}},
			}

			repository, err := name.NewRepository(fmt.Sprintf("%s/k8s/%s", strings.TrimPrefix(registryServer.URL, "http://"), PropagationName))
			Expect(err).NotTo(HaveOccurred())

			ociClient := clients.NewOCIPropagationBackendClient(repository)
			propagationConfigClient := clients.NewPropagationClient(&ociClient)
			Expect(propagationConfigClient.PublishConfig(config)).To(Succeed())

			// Set initial deployment versions
			pushImage(devRev2Image.image, PropagationName, "deploy", "frankfurt-dev-1", "latest")
			pushImage(stagingRev1Image.image, PropagationName, "deploy", "frankfurt-staging-1", "latest")

			By("By creating a new Propagation")

			createPropagation(ctx, PropagationName, PropagationDevNamespace, "frankfurt-dev-1")

			propagationDevLookupKey := types.NamespacedName{Name: PropagationName, Namespace: PropagationDevNamespace}
			createdPropagationDev := &v1alpha1.Propagation{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, propagationDevLookupKey, createdPropagationDev)
				if err != nil {
					return false
				}
				for _, c := range createdPropagationDev.Status.Conditions {
					if c.Type == "Ready" && c.Status == metav1.ConditionFalse {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("By creating a new ConfigMap with deployed version to dev")
			createDeployedVersionConfigMap(PropagationName, PropagationDevNamespace, "rev-2")

			By("Reading deployed version from referenced ConfigMap marks Propagation as ready")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, propagationDevLookupKey, createdPropagationDev)
				if err != nil {
					return false
				}
				for _, c := range createdPropagationDev.Status.Conditions {
					if c.Type == "Ready" && c.Status == metav1.ConditionTrue {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Propagation reports status of the current deployment")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, propagationDevLookupKey, createdPropagationDev)
				if err != nil {
					return false
				}
				return createdPropagationDev.Status.DeploymentStatus.Version == "rev-2"
			}, timeout, interval).Should(BeTrue())

			By("By creating a staging Propagation depending on the first one")

			createPropagation(ctx, PropagationName, PropagationStagingNamespace, "frankfurt-staging-1")

			By("By creating a new ConfigMap with deployed version of staging Propagation")

			createDeployedVersionConfigMap(PropagationName, PropagationStagingNamespace, "rev-1")

			By("Updating deployment status reports")
			propagationStagingLookupKey := types.NamespacedName{Name: PropagationName, Namespace: PropagationStagingNamespace}
			createdPropagationStaging := &v1alpha1.Propagation{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, propagationStagingLookupKey, createdPropagationStaging)
				if err != nil {
					return false
				}

				for _, r := range createdPropagationStaging.Status.DeploymentStatusesReports {
					if r.DeploymentName == "frankfurt-dev-1" {
						for _, s := range r.Statuses {
							if s.Version == "rev-2" && s.State == "Healthy" {
								return true
							}
						}
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Waiting for version rev-1 to be healthy for specified duration")

			Consistently(func() bool {
				return assertPropagatedDeployment(PropagationName, "frankfurt-staging-1", stagingRev1Image)
			}, timeout, interval).Should(BeTrue())

			createdPropagationStaging.Status.DeploymentStatusesReports[0].LastStatus().Start = metav1.NewTime(
				time.Now().Add(-59 * time.Minute),
			)
			Expect(k8sClient.Status().Update(ctx, createdPropagationStaging)).Should(Succeed())

			Consistently(func() bool {
				return assertPropagatedDeployment(PropagationName, "frankfurt-staging-1", stagingRev1Image)
			}, timeout, interval).Should(BeTrue())

			By("Propagating version")
			createdPropagationStaging.Status.DeploymentStatusesReports[0].LastStatus().Start = metav1.NewTime(time.Now().Add(-time.Hour))
			Expect(k8sClient.Status().Update(ctx, createdPropagationStaging)).Should(Succeed())

			Eventually(func() bool {
				return assertPropagatedDeployment(PropagationName, "frankfurt-staging-1", stagingRev2Image)
			}, timeout, interval).Should(BeTrue())

			By("Starting a new propagation version in dev")
			Consistently(func() bool { return assertPropagatedDeployment(PropagationName, "frankfurt-dev-1", devRev2Image) }, timeout, interval).Should(BeTrue())

			devRev3Image := pushFakeDeploymentImage(PropagationName, "frankfurt-dev-1", "rev-3")

			Eventually(func() bool { return assertPropagatedDeployment(PropagationName, "frankfurt-dev-1", devRev3Image) }, timeout, interval).Should(BeTrue())
			Consistently(func() bool {
				return assertPropagatedDeployment(PropagationName, "frankfurt-staging-1", stagingRev2Image)
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When propagating 3 waves across the same environment", func() {
		const (
			PropagationName           = "prod-3-waves"
			PropagationProd1Namespace = "prod-1"
			PropagationProd2Namespace = "prod-2"
			PropagationProd3Namespace = "prod-3"
		)
		It("Should propagate versions to/from multiple deployments", func() {
			ctx := context.Background()

			By("Pushing manifests")
			prod1Rev2Image := pushFakeDeploymentImage(PropagationName, "prod-1", "rev-2")
			_ = prod1Rev2Image
			prod2Rev1Image := pushFakeDeploymentImage(PropagationName, "prod-2", "rev-1")
			_ = prod2Rev1Image
			prod2Rev2Image := pushFakeDeploymentImage(PropagationName, "prod-2", "rev-2")
			_ = prod2Rev2Image
			prod3Rev1Image := pushFakeDeploymentImage(PropagationName, "prod-3", "rev-1")
			_ = prod3Rev1Image
			prod3Rev2Image := pushFakeDeploymentImage(PropagationName, "prod-3", "rev-2")
			_ = prod3Rev2Image

			config := config.Config{
				Environments: []config.Environment{{
					Name: "prod",
					Waves: []config.Wave{{
						BakeTime:    metav1.Duration{Duration: 1 * time.Hour},
						Deployments: []string{"prod-1"},
					}, {
						BakeTime:    metav1.Duration{Duration: 1 * time.Hour},
						Deployments: []string{"prod-2"},
					}, {
						BakeTime:    metav1.Duration{Duration: 1 * time.Hour},
						Deployments: []string{"prod-3"},
					}},
				}},
			}

			repository, err := name.NewRepository(fmt.Sprintf("%s/k8s/%s", strings.TrimPrefix(registryServer.URL, "http://"), PropagationName))
			Expect(err).NotTo(HaveOccurred())

			ociClient := clients.NewOCIPropagationBackendClient(repository)
			propagationConfigClient := clients.NewPropagationClient(&ociClient)
			Expect(propagationConfigClient.PublishConfig(config)).To(Succeed())

			// Set initial deployment versions
			pushImage(prod1Rev2Image.image, PropagationName, "deploy", "prod-1", "latest")
			pushImage(prod2Rev1Image.image, PropagationName, "deploy", "prod-2", "latest")
			pushImage(prod3Rev1Image.image, PropagationName, "deploy", "prod-3", "latest")

			By("By creating propagations and version ConfigMaps for each deployment")

			createPropagation(ctx, PropagationName, PropagationProd1Namespace, "prod-1")
			createPropagation(ctx, PropagationName, PropagationProd2Namespace, "prod-2")
			createPropagation(ctx, PropagationName, PropagationProd3Namespace, "prod-3")

			createDeployedVersionConfigMap(PropagationName, PropagationProd1Namespace, "rev-2")
			createDeployedVersionConfigMap(PropagationName, PropagationProd2Namespace, "rev-1")
			createDeployedVersionConfigMap(PropagationName, PropagationProd3Namespace, "rev-1")

			By("prod-1 waiting for prod-2 and prod-3 to reach its version when there's a new version available")
			prod1Rev3Image := pushFakeDeploymentImage(PropagationName, "prod-1", "rev-3")
			pushFakeDeploymentImage(PropagationName, "prod-2", "rev-3")
			pushFakeDeploymentImage(PropagationName, "prod-3", "rev-3")

			// check that prod-1 is still at rev-2
			Consistently(func() bool {
				return assertPropagatedDeployment(PropagationName, "prod-1", prod1Rev2Image) && assertPropagatedDeployment(PropagationName, "prod-2", prod2Rev1Image) && assertPropagatedDeployment(PropagationName, "prod-3", prod3Rev1Image)
			}, timeout, interval).Should(BeTrue())

			By("prod-2 upgrading to rev-2 after prod-1 is healthy for enough time")
			propagationProd2LookupKey := types.NamespacedName{Name: PropagationName, Namespace: PropagationProd2Namespace}
			createdPropagationProd2 := &v1alpha1.Propagation{}
			Eventually(func() error {
				return k8sClient.Get(ctx, propagationProd2LookupKey, createdPropagationProd2)
			}, timeout, interval).Should(Succeed())
			for _, r := range createdPropagationProd2.Status.DeploymentStatusesReports {
				if r.DeploymentName == "prod-1" {
					r.LastStatus().Start = metav1.NewTime(time.Now().Add(-time.Hour))
				}
			}
			Expect(k8sClient.Status().Update(ctx, createdPropagationProd2)).Should(Succeed())

			Eventually(func() bool {
				return assertPropagatedDeployment(PropagationName, "prod-1", prod1Rev2Image) && assertPropagatedDeployment(PropagationName, "prod-2", prod2Rev2Image) && assertPropagatedDeployment(PropagationName, "prod-3", prod3Rev1Image)
			}, timeout, interval).Should(BeTrue())

			Eventually(func() error {
				return updateDeployedVersionConfigMap(PropagationName, PropagationProd2Namespace, "rev-2")
			}, timeout, interval).Should(Succeed())

			By("prod-1 and prod-2 waiting for prod-3 to reach its version when there's a new version available")
			Consistently(func() bool {
				return assertPropagatedDeployment(PropagationName, "prod-1", prod1Rev2Image) && assertPropagatedDeployment(PropagationName, "prod-2", prod2Rev2Image) && assertPropagatedDeployment(PropagationName, "prod-3", prod3Rev1Image)
			}, timeout, interval).Should(BeTrue())

			propagationProd3LookupKey := types.NamespacedName{Name: PropagationName, Namespace: PropagationProd3Namespace}
			createdPropagationProd3 := &v1alpha1.Propagation{}
			Eventually(func() error {
				return k8sClient.Get(ctx, propagationProd3LookupKey, createdPropagationProd3)
			}, timeout, interval).Should(Succeed())
			for _, r := range createdPropagationProd3.Status.DeploymentStatusesReports {
				if r.DeploymentName == "prod-2" {
					r.LastStatus().Start = metav1.NewTime(time.Now().Add(-time.Hour))
				}
			}
			Expect(k8sClient.Status().Update(ctx, createdPropagationProd3)).Should(Succeed())

			Eventually(func() bool {
				return assertPropagatedDeployment(PropagationName, "prod-1", prod1Rev2Image) && assertPropagatedDeployment(PropagationName, "prod-2", prod2Rev2Image) && assertPropagatedDeployment(PropagationName, "prod-3", prod3Rev2Image)
			}, timeout, interval).Should(BeTrue())

			Eventually(func() error {
				return updateDeployedVersionConfigMap(PropagationName, PropagationProd3Namespace, "rev-2")
			}, timeout, interval).Should(Succeed())

			By("prod-1 upgrading to new rev-3")
			Consistently(func() bool {
				return assertPropagatedDeployment(PropagationName, "prod-1", prod1Rev2Image) && assertPropagatedDeployment(PropagationName, "prod-2", prod2Rev2Image) && assertPropagatedDeployment(PropagationName, "prod-3", prod3Rev2Image)
			}, timeout, interval).Should(BeTrue())

			propagationProd1LookupKey := types.NamespacedName{Name: PropagationName, Namespace: PropagationProd1Namespace}
			createdPropagationProd1 := &v1alpha1.Propagation{}
			Eventually(func() error {
				return k8sClient.Get(ctx, propagationProd1LookupKey, createdPropagationProd1)
			}, timeout, interval).Should(Succeed())
			for _, r := range createdPropagationProd1.Status.DeploymentStatusesReports {
				if slices.Contains([]string{"prod-2", "prod-3"}, r.DeploymentName) {
					r.LastStatus().Start = metav1.NewTime(time.Now().Add(-time.Hour))
				}
			}
			Expect(k8sClient.Status().Update(ctx, createdPropagationProd1)).Should(Succeed())

			Eventually(func() bool {
				return assertPropagatedDeployment(PropagationName, "prod-1", prod1Rev3Image) && assertPropagatedDeployment(PropagationName, "prod-2", prod2Rev2Image) && assertPropagatedDeployment(PropagationName, "prod-3", prod3Rev2Image)
			}, timeout, interval).Should(BeTrue())
			Consistently(func() bool {
				return assertPropagatedDeployment(PropagationName, "prod-1", prod1Rev3Image) && assertPropagatedDeployment(PropagationName, "prod-2", prod2Rev2Image) && assertPropagatedDeployment(PropagationName, "prod-3", prod3Rev2Image)
			}, timeout, interval).Should(BeTrue())

		})
	})
})

type testImage struct {
	image   v1.Image
	digest  v1.Hash
	version string
}

func pushFakeDeploymentImage(propagationName, deployment, version string) testImage {
	image, err := mutate.AppendLayers(empty.Image, static.NewLayer([]byte(fmt.Sprintf("%s/%s", deployment, version)), typescrane.MediaType("fake")))
	Expect(err).ShouldNot(HaveOccurred())
	digest, err := image.Digest()
	Expect(err).ShouldNot(HaveOccurred())
	fmt.Printf("Fake deploy image for deployment %s, version %s has digest %s\n", deployment, version, digest)
	pushImage(image, propagationName, "manifests", deployment, version)
	pushImage(image, propagationName, "manifests", deployment, "latest")
	return testImage{
		image:   image,
		digest:  digest,
		version: version,
	}
}

func pushImage(image v1.Image, propagationName, artifactType, deployment, tag string) {
	imageURL := fmt.Sprintf("%s/k8s/%s/%s/%s:%s", registryEndpoint, propagationName, artifactType, deployment, tag)
	fmt.Println("Pushing manifest to", imageURL)
	Expect(
		crane.Push(image, imageURL),
	).To(Succeed())
}

func createPropagation(ctx context.Context, propagationName, propagationNamespace, name string) {
	Expect(k8sClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: propagationNamespace,
		},
	})).Should(Succeed())
	Eventually(func() error {
		return k8sClient.Get(ctx, types.NamespacedName{Name: propagationNamespace}, &corev1.Namespace{})
	}).Should(Succeed())

	propagation := &v1alpha1.Propagation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kuberik.io/v1alpha1",
			Kind:       "Propagation",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      propagationName,
			Namespace: propagationNamespace,
		},
		Spec: v1alpha1.PropagationSpec{
			Backend: v1alpha1.PropagationBackend{
				BaseUrl: fmt.Sprintf("oci://%s/k8s/%s", registryEndpoint, propagationName),
			},
			Deployment: v1alpha1.Deployment{
				Name: name,
				Version: v1alpha1.LocalObjectField{
					APIVersion: "v1",
					Kind:       "ConfigMap",
					Name:       propagationName,
					FieldPath:  "data.version",
				},
			},
			PollInterval: metav1.Duration{Duration: time.Second},
		},
	}
	Expect(k8sClient.Create(ctx, propagation)).Should(Succeed())
}

func createDeployedVersionConfigMap(propagationName, propagationNamespace, version string) {
	deployedVersionConfigMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      propagationName,
			Namespace: propagationNamespace,
		},
		Data: map[string]string{
			"version": version,
		},
	}
	Expect(k8sClient.Create(ctx, deployedVersionConfigMap)).Should(Succeed())
}

func updateDeployedVersionConfigMap(propagationName, propagationNamespace, version string) error {
	lookupKey := types.NamespacedName{Name: propagationName, Namespace: propagationNamespace}
	cm := &corev1.ConfigMap{}
	err := k8sClient.Get(ctx, lookupKey, cm)
	if err != nil {
		return err
	}
	cm.Data["version"] = version
	err = k8sClient.Update(ctx, cm)
	return err
}

func assertPropagatedDeployment(propagationName, deployment string, want testImage) bool {
	currentDeployImage, err := crane.Pull(fmt.Sprintf("%s/k8s/%s/deploy/%s:latest", registryEndpoint, propagationName, deployment))
	if err != nil {
		return false
	}
	digest, err := currentDeployImage.Digest()
	if err != nil {
		return false
	}
	return digest.String() == want.digest.String()
}
