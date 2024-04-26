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
		PropagationName             = "my-app"
		PropagationDevNamespace     = "dev"
		PropagationStagingNamespace = "staging"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When updating Propagation Status", func() {
		It("Should propagate version to/from another deployment", func() {
			ctx := context.Background()

			By("Pushing manifests")
			fakeDeploymentImage := func(deployment, version string) (v1.Image, error) {
				return mutate.AppendLayers(empty.Image, static.NewLayer([]byte(fmt.Sprintf("%s/%s", deployment, version)), typescrane.MediaType("fake")))
			}
			devRev2Image, err := fakeDeploymentImage("frankfurt-dev-1", "rev-2")
			Expect(err).ShouldNot(HaveOccurred())
			stagingRev1Image, err := fakeDeploymentImage("frankfurt-staging-1", "rev-1")
			Expect(err).ShouldNot(HaveOccurred())
			stagingRev1ImageDigest, err := stagingRev1Image.Digest()
			Expect(err).Should(Not(HaveOccurred()))
			stagingRev2Image, err := fakeDeploymentImage("frankfurt-staging-1", "rev-2")
			Expect(err).ShouldNot(HaveOccurred())
			stagingRev2ImageDigest, err := stagingRev2Image.Digest()
			Expect(err).Should(Not(HaveOccurred()))

			config := config.Config{
				Environments: []config.Environment{{
					Name: "dev",
					Waves: []config.Wave{{
						BakeTime:    metav1.Duration{Duration: 1 * time.Hour},
						Deployments: []string{"frankfurt-dev-1"},
					}},
					// TODO: not implemented
					ReleaseCadence: config.ReleaseCadence{
						WaitTime: metav1.Duration{Duration: 2 * time.Hour},
					},
				}, {
					Name: "staging",
					Waves: []config.Wave{{
						BakeTime:    metav1.Duration{Duration: 4 * time.Hour},
						Deployments: []string{"frankfurt-staging-1"},
					}},
					// TODO: not implemented
					ReleaseCadence: config.ReleaseCadence{
						// At 08:00 on Monday.
						Schedule: "0 8 * * 1",
					},
				}},
			}

			repository, err := name.NewRepository(fmt.Sprintf("%s/k8s/%s", strings.TrimPrefix(registryServer.URL, "http://"), PropagationName))
			Expect(err).NotTo(HaveOccurred())

			ociClient := clients.NewOCIPropagationBackendClient(repository)
			propagationConfigClient := clients.NewPropagationConfigClient(&ociClient)
			Expect(propagationConfigClient.PublishConfig(config)).To(Succeed())

			// TODO: replace with a real client that will be used in CI
			Expect(
				crane.Push(devRev2Image, fmt.Sprintf("%s/k8s/my-app/manifests/frankfurt-dev-1:rev-2", registryEndpoint)),
			).To(Succeed())
			Expect(
				crane.Push(stagingRev1Image, fmt.Sprintf("%s/k8s/my-app/manifests/frankfurt-staging-1:rev-1", registryEndpoint)),
			).To(Succeed())
			Expect(
				crane.Push(stagingRev2Image, fmt.Sprintf("%s/k8s/my-app/manifests/frankfurt-staging-1:rev-2", registryEndpoint)),
			).To(Succeed())

			Expect(
				crane.Push(devRev2Image, fmt.Sprintf("%s/k8s/my-app/deploy/frankfurt-dev-1:latest", registryEndpoint)),
			).To(Succeed())
			Expect(
				crane.Push(stagingRev1Image, fmt.Sprintf("%s/k8s/my-app/deploy/frankfurt-staging-1:latest", registryEndpoint)),
			).To(Succeed())

			By("By creating a new Propagation")

			Expect(k8sClient.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: PropagationDevNamespace,
				},
			})).Should(Succeed())
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: PropagationDevNamespace}, &corev1.Namespace{})
			}).Should(Succeed())

			propagationDev := &v1alpha1.Propagation{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kuberik.io/v1alpha1",
					Kind:       "Propagation",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      PropagationName,
					Namespace: PropagationDevNamespace,
				},
				Spec: v1alpha1.PropagationSpec{
					Backend: v1alpha1.PropagationBackend{
						BaseUrl: fmt.Sprintf("oci://%s/k8s/my-app", registryEndpoint),
					},
					Deployment: v1alpha1.Deployment{
						Name:        "frankfurt-dev-1",
						Environment: "dev",
						Wave:        1,
						Version: v1alpha1.LocalObjectField{
							APIVersion: "v1",
							Kind:       "ConfigMap",
							Name:       PropagationName,
							FieldPath:  "data.version",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, propagationDev)).Should(Succeed())

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
			deployedVersionDevConfigMap := &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      PropagationName,
					Namespace: PropagationDevNamespace,
				},
				Data: map[string]string{
					"version": "rev-2",
				},
			}
			Expect(k8sClient.Create(ctx, deployedVersionDevConfigMap)).Should(Succeed())

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

			Expect(k8sClient.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: PropagationStagingNamespace,
				},
			})).Should(Succeed())
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: PropagationStagingNamespace}, &corev1.Namespace{})
			}).Should(Succeed())

			propagationStaging := &v1alpha1.Propagation{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kuberik.io/v1alpha1",
					Kind:       "Propagation",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      PropagationName,
					Namespace: PropagationStagingNamespace,
				},
				Spec: v1alpha1.PropagationSpec{
					Backend: v1alpha1.PropagationBackend{
						BaseUrl: fmt.Sprintf("oci://%s/k8s/my-app", registryEndpoint),
					},
					Deployment: v1alpha1.Deployment{
						Name:        "frankfurt-staging-1",
						Environment: "staging",
						Wave:        1,
						Version: v1alpha1.LocalObjectField{
							APIVersion: "v1",
							Kind:       "ConfigMap",
							Name:       PropagationName,
							FieldPath:  "data.version",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, propagationStaging)).Should(Succeed())

			By("By creating a new ConfigMap with deployed version of staging Propagation")
			deployedStagingVersionConfigMap := &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      PropagationName,
					Namespace: PropagationStagingNamespace,
				},
				Data: map[string]string{
					"version": "rev-1",
				},
			}
			Expect(k8sClient.Create(ctx, deployedStagingVersionConfigMap)).Should(Succeed())

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

			Consistently(func() (v1.Hash, error) {
				currentDeployImage, err := crane.Pull(fmt.Sprintf("%s/k8s/my-app/deploy/frankfurt-staging-1:latest", registryEndpoint))
				if err != nil {
					return v1.Hash{}, err
				}
				return currentDeployImage.Digest()
			}, timeout, interval).Should(Equal(stagingRev1ImageDigest))

			createdPropagationStaging.Status.DeploymentStatusesReports[0].Statuses[0].Start = metav1.NewTime(
				time.Now().Add(-59 * time.Minute),
			)
			Expect(k8sClient.Status().Update(ctx, createdPropagationStaging)).Should(Succeed())

			Consistently(func() (v1.Hash, error) {
				currentDeployImage, err := crane.Pull(fmt.Sprintf("%s/k8s/my-app/deploy/frankfurt-staging-1:latest", registryEndpoint))
				if err != nil {
					return v1.Hash{}, err
				}
				return currentDeployImage.Digest()
			}, timeout, interval).Should(Equal(stagingRev1ImageDigest))

			By("Propagating version")
			createdPropagationStaging.Status.DeploymentStatusesReports[0].Statuses[0].Start = metav1.NewTime(time.Now().Add(-time.Hour))
			Expect(k8sClient.Status().Update(ctx, createdPropagationStaging)).Should(Succeed())

			Eventually(func() (v1.Hash, error) {
				currentDeployImage, err := crane.Pull(fmt.Sprintf("%s/k8s/my-app/deploy/frankfurt-staging-1:latest", registryEndpoint))
				if err != nil {
					return v1.Hash{}, err
				}
				return currentDeployImage.Digest()
			}, timeout, interval).Should(Equal(stagingRev2ImageDigest))

			// TODO: test dev gets propagated from latest
		})
	})

})
