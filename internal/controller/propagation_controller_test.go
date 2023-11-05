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
	"time"

	"github.com/kuberik/propagation-controller/api/v1alpha1"
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
		It("Should set Propagation's health report in Status. Health report should be published to the backend.", func() {

			ctx := context.Background()

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
						BaseUrl: "oci://registry.local/k8s",
					},
					Deployment: v1alpha1.Deployment{
						Name: "frankfurt-dev-1",
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

			By("By creating a new ConfigMap with deployed version")
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
						BaseUrl: "oci://registry.local/k8s",
					},
					Deployment: v1alpha1.Deployment{
						Name: "frankfurt-staging-1",
						Version: v1alpha1.LocalObjectField{
							APIVersion: "v1",
							Kind:       "ConfigMap",
							Name:       PropagationName,
							FieldPath:  "data.version",
						},
					},
					DeployAfter: v1alpha1.DeployAfter{
						Deployments: []string{
							"frankfurt-dev-1",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, propagationStaging)).Should(Succeed())

			By("By creating a new ConfigMap with deployed version of Propagation two")
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

			// TODO: test version change
		})
	})

})
