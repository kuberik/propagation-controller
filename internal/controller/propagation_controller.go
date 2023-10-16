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

	v1alpha1 "github.com/kuberik/propagation-controller/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	kyaml_utils "sigs.k8s.io/kustomize/kyaml/utils"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// PropagationReconciler reconciles a Propagation object
type PropagationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kuberik.io,resources=propagations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kuberik.io,resources=propagations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kuberik.io,resources=propagations/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PropagationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	propagation := &v1alpha1.Propagation{}
	err := r.Client.Get(ctx, req.NamespacedName, propagation)
	if err != nil {
		return ctrl.Result{}, err
	}

	readyCondition := meta.FindStatusCondition(propagation.Status.Conditions, v1alpha1.ReadyCondition)
	if readyCondition == nil {
		readyCondition = &metav1.Condition{
			Type: v1alpha1.ReadyCondition,
		}
		propagation.Status.Conditions = append(propagation.Status.Conditions, *readyCondition)
		readyCondition = &propagation.Status.Conditions[len(propagation.Status.Conditions)-1]
	}
	readyCondition.Reason = "getVersion"
	version, err := r.getVersion(ctx, *propagation)
	if err != nil {
		readyCondition.LastTransitionTime = metav1.Now()
		readyCondition.Message = err.Error()
		readyCondition.Status = metav1.ConditionFalse
		r.Client.Status().Update(ctx, propagation)
		return ctrl.Result{}, err
	} else if readyCondition.Status != metav1.ConditionTrue {
		readyCondition.Status = metav1.ConditionTrue
		readyCondition.LastTransitionTime = metav1.Now()
		readyCondition.Message = "Successfully parsed version"
		if err := r.Client.Status().Update(ctx, propagation); err != nil {
			return ctrl.Result{}, err
		}
	}

	// TODO: setting to healthy always for starters because we don't have any health controllers and will at least make controller somewhat useful
	deploymentState := v1alpha1.HealthStateHealthy
	if propagation.Status.DeploymentStatus.Version != version ||
		propagation.Status.DeploymentStatus.State != deploymentState {
		propagation.Status.DeploymentStatus = v1alpha1.DeploymentStatus{
			Version: version,
			State:   v1alpha1.HealthStateHealthy,
		}

		// TODO: publish status to oci or s3 backend

		err = r.Client.Status().Update(ctx, propagation)
		if err != nil {
			return ctrl.Result{}, nil
		}
	}

	// if status healthy, propagate
	// 1. fetch statuses
	// 2. get first version thtat satisfies all the requirements
	// 3. if no version found return
	// 4. propagate version

	return ctrl.Result{}, nil
}

func (r *PropagationReconciler) getVersion(ctx context.Context, propagation v1alpha1.Propagation) (string, error) {
	versionObject := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": propagation.Spec.Deployment.Version.APIVersion,
			"kind":       propagation.Spec.Deployment.Version.Kind,
		},
	}
	if propagation.Spec.Deployment.Version.Name == "" {
		return "", fmt.Errorf("missing version's object name")
	}
	if propagation.Spec.Deployment.Version.FieldPath == "" {
		return "", fmt.Errorf("missing version's object fieldPath")
	}
	err := r.Client.Get(ctx, types.NamespacedName{Name: propagation.Spec.Deployment.Version.Name, Namespace: propagation.Namespace}, versionObject)
	if err != nil {
		return "", err
	}

	objRNode, err := yaml.FromMap(versionObject.Object)
	if err != nil {
		return "", err
	}

	fieldPath := kyaml_utils.SmarterPathSplitter(propagation.Spec.Deployment.Version.FieldPath, ".")
	rn, err := objRNode.Pipe(yaml.Lookup(fieldPath...))
	if err != nil {
		return "", fmt.Errorf("error looking up version path: %w", err)
	}
	if rn.IsNilOrEmpty() {
		return "", fmt.Errorf("version is empty")
	}

	if !rn.IsStringValue() {
		return "", fmt.Errorf("version field is not a string")
	}
	return rn.Document().Value, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PropagationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Propagation{}).
		Complete(r)
}
