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
	"testing"
	"time"

	v1alpha1 "github.com/kuberik/propagation-controller/api/v1alpha1"
	"github.com/kuberik/propagation-controller/pkg/repo/config"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetVersion(t *testing.T) {
	validPropagation := v1alpha1.Propagation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-propagation",
			Namespace: "my-app-namespace",
		},
		Spec: v1alpha1.PropagationSpec{
			Deployment: v1alpha1.Deployment{
				Version: v1alpha1.LocalObjectField{
					Kind:       "ConfigMap",
					APIVersion: "v1",
					Name:       "my-app-version",
					FieldPath:  "data.app-version",
				},
			},
		},
	}
	testCases := []struct {
		name         string
		propagation  v1alpha1.Propagation
		createObject client.Object
		version      string
		errorMessage string
	}{{
		name: "version object reference missing kind",
		propagation: v1alpha1.Propagation{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-propagation",
				Namespace: "my-app-namespace",
			},
			Spec: v1alpha1.PropagationSpec{
				Deployment: v1alpha1.Deployment{
					Version: v1alpha1.LocalObjectField{
						APIVersion: "v1",
						Name:       "my-app-version",
						FieldPath:  "data.app-version",
					},
				},
			},
		},
		createObject: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-app-version",
			},
		},
		errorMessage: "Object 'Kind' is missing in 'unstructured object has no kind'",
	}, {
		name: "version object reference missing apiVersion",
		propagation: v1alpha1.Propagation{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-propagation",
				Namespace: "my-app-namespace",
			},
			Spec: v1alpha1.PropagationSpec{
				Deployment: v1alpha1.Deployment{
					Version: v1alpha1.LocalObjectField{
						Kind:      "ConfigMap",
						Name:      "my-app-version",
						FieldPath: "data.app-version",
					},
				},
			},
		},
		createObject: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-version",
				Namespace: "my-app-namespace",
			},
		},
		errorMessage: "Object 'apiVersion' is missing in 'unstructured object has no version'",
	}, {
		name: "version object reference missing name",
		propagation: v1alpha1.Propagation{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-propagation",
				Namespace: "my-app-namespace",
			},
			Spec: v1alpha1.PropagationSpec{
				Deployment: v1alpha1.Deployment{
					Version: v1alpha1.LocalObjectField{
						Kind:       "ConfigMap",
						APIVersion: "v1",
						FieldPath:  "data.app-version",
					},
				},
			},
		},
		createObject: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-version",
				Namespace: "my-app-namespace",
			},
		},
		errorMessage: "missing version's object name",
	}, {
		name: "version object reference missing fieldPath",
		propagation: v1alpha1.Propagation{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-propagation",
				Namespace: "my-app-namespace",
			},
			Spec: v1alpha1.PropagationSpec{
				Deployment: v1alpha1.Deployment{
					Version: v1alpha1.LocalObjectField{
						Kind:       "ConfigMap",
						APIVersion: "v1",
						Name:       "my-app-version",
					},
				},
			},
		},
		createObject: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-version",
				Namespace: "my-app-namespace",
			},
		},
		errorMessage: "missing version's object fieldPath",
	}, {
		name:        "version object wrong namespace",
		propagation: validPropagation,
		createObject: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-app-version",
			},
		},
		errorMessage: "configmaps \"my-app-version\" not found",
	}, {
		name:        "version object wrong name",
		propagation: validPropagation,
		createObject: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bar",
				Namespace: "my-app-namespace",
			},
		},
		errorMessage: "configmaps \"my-app-version\" not found",
	}, {
		name:        "version missing version field",
		propagation: validPropagation,
		createObject: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-version",
				Namespace: "my-app-namespace",
			},
		},
		errorMessage: "version is empty",
	}, {
		name:        "version field empty",
		propagation: validPropagation,
		createObject: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-version",
				Namespace: "my-app-namespace",
			},
		},
		errorMessage: "version is empty",
	}, {
		name: "version field is a number",
		propagation: v1alpha1.Propagation{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-propagation",
				Namespace: "my-app-namespace",
			},
			Spec: v1alpha1.PropagationSpec{
				Deployment: v1alpha1.Deployment{
					Version: v1alpha1.LocalObjectField{
						APIVersion: "v1",
						Kind:       "ConfigMap",
						Name:       "my-app-version",
						FieldPath:  "metadata.generation",
					},
				},
			},
		},
		createObject: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "my-app-version",
				Namespace:  "my-app-namespace",
				Generation: 7,
			},
		},
		errorMessage: "version field is not a string",
	}, {
		name: "version field is an object",
		propagation: v1alpha1.Propagation{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-propagation",
				Namespace: "my-app-namespace",
			},
			Spec: v1alpha1.PropagationSpec{
				Deployment: v1alpha1.Deployment{
					Version: v1alpha1.LocalObjectField{
						APIVersion: "v1",
						Kind:       "ConfigMap",
						Name:       "my-app-version",
						FieldPath:  "metadata",
					},
				},
			},
		},
		createObject: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-version",
				Namespace: "my-app-namespace",
			},
		},
		errorMessage: "version field is not a string",
	}, {
		name:        "version object valid",
		propagation: validPropagation,
		createObject: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-version",
				Namespace: "my-app-namespace",
			},
			Data: map[string]string{
				"app-version": "1234",
			},
		},
		version: "1234",
	}}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithObjects(tc.createObject).Build()
			reconciler := PropagationReconciler{
				Client: client,
			}

			version, err := reconciler.getVersion(context.TODO(), tc.propagation)
			var errorMessage string
			if err != nil {
				errorMessage = err.Error()
			}
			assert.Equal(t, tc.errorMessage, errorMessage)
			assert.Equal(t, tc.version, version)

		})
	}
}

func TestSetDeployAfterFromConfig(t *testing.T) {
	configFull := config.Config{
		Environments: []config.Environment{{
			Name: "dev",
			Waves: []config.Wave{{
				BakeTime: metav1.Duration{Duration: time.Hour},
				Deployments: []string{
					"frankfurt-dev-1",
				},
			}},
			ReleaseCadence: config.ReleaseCadence{
				WaitTime: metav1.Duration{Duration: 2 * time.Hour},
			},
		}, {
			Name: "production",
			Waves: []config.Wave{{
				BakeTime: metav1.Duration{Duration: time.Hour},
				Deployments: []string{
					"frankfurt-production-1",
				},
			}},
			ReleaseCadence: config.ReleaseCadence{
				// At 08:00 on Monday.
				Schedule: "0 8 * * 1",
			},
		}},
	}

	testCases := []struct {
		name    string
		input   v1alpha1.Deployment
		want    v1alpha1.DeployAfter
		config  config.Config
		wantErr bool
	}{{
		name:   "full config prod",
		config: configFull,
		input: v1alpha1.Deployment{
			Name: "frankfurt-production-1",
		},
		want: v1alpha1.DeployAfter{
			Deployments: []string{
				"frankfurt-dev-1",
			},
			BakeTime: metav1.Duration{Duration: time.Hour},
		},
	}, {
		name:   "full config missing deployment",
		config: configFull,
		input: v1alpha1.Deployment{
			Name: "frankfurt-production-2",
		},
		wantErr: true,
	}, {
		name:   "full config dev",
		config: configFull,
		input: v1alpha1.Deployment{
			Name: "frankfurt-dev-1",
		},
		want: v1alpha1.DeployAfter{},
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			deployAfter, err := deployAfterFromConfig(v1alpha1.Propagation{
				Spec: v1alpha1.PropagationSpec{
					Deployment: tc.input,
				},
			}, tc.config)
			assert.Equal(t, err != nil, tc.wantErr)
			if !tc.wantErr {
				assert.Equal(t, tc.want, *deployAfter)
			} else {
				assert.Nil(t, deployAfter)
			}
		})
	}

}

func TestDeployWithFromConfig(t *testing.T) {
	configFull := config.Config{
		Environments: []config.Environment{{
			Name: "dev",
			Waves: []config.Wave{{
				Deployments: []string{
					"frankfurt-dev-1",
				},
			}},
		}, {
			Name: "production",
			Waves: []config.Wave{{
				Deployments: []string{
					"frankfurt-production-1",
				},
			}, {
				Deployments: []string{
					"frankfurt-production-2",
				},
			}, {
				Deployments: []string{
					"frankfurt-production-3",
					"frankfurt-production-4",
				},
			}},
		}},
	}

	testCases := []struct {
		input   string
		want    []string
		config  config.Config
		wantErr bool
	}{{
		config: configFull,
		input:  "frankfurt-dev-1",
		want:   []string{},
	}, {
		config: configFull,
		input:  "frankfurt-production-1",
		want: []string{
			"frankfurt-production-2",
			"frankfurt-production-3",
			"frankfurt-production-4",
		},
	}, {
		config: configFull,
		input:  "frankfurt-production-2",
		want: []string{
			"frankfurt-production-3",
			"frankfurt-production-4",
		},
	}, {
		config: configFull,
		input:  "frankfurt-production-3",
		want:   []string{},
	}, {
		config: configFull,
		input:  "frankfurt-production-4",
		want:   []string{},
	}, {
		config:  configFull,
		input:   "frankfurt-production-5",
		wantErr: true,
	}}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			deployWith, err := deployWithFromConfig(v1alpha1.Propagation{
				Spec: v1alpha1.PropagationSpec{
					Deployment: v1alpha1.Deployment{
						Name: tc.input,
					},
				},
			}, tc.config)
			assert.Equal(t, err != nil, tc.wantErr)
			if !tc.wantErr {
				assert.Equal(t, tc.want, deployWith)
			} else {
				assert.Nil(t, deployWith)
			}
		})
	}
}
