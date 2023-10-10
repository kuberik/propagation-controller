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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/kuberik/propagation-controller/api/v1alpha1"
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

	// propagation := v1alpha1.Propagation{}
	// err := r.Client.Get(ctx, req.NamespacedName, &propagation)
	// update status
	// 1. get version
	// 2. get statuses and aggregate (and over all)
	// 3. publish status

	// if status healthy, propagate
	// 1. fetch statuses
	// 2. get first version thtat satisfies all the requirements
	// 3. if no version found return
	// 4. propagate version

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PropagationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Propagation{}).
		Complete(r)
}
