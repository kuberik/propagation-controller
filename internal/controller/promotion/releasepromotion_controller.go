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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/kuberik/propagation-controller/api/promotion/v1alpha1"
	promotionv1alpha1 "github.com/kuberik/propagation-controller/api/promotion/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReleasePromotionReconciler reconciles a ReleasePromotion object
type ReleasePromotionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=promotion.kuberik.io,resources=releasepromotions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=promotion.kuberik.io,resources=releasepromotions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=promotion.kuberik.io,resources=releasepromotions/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ReleasePromotion object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.4/pkg/reconcile
func (r *ReleasePromotionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)

	releasePromotion := v1alpha1.ReleasePromotion{}
	if err := r.Client.Get(ctx, req.NamespacedName, &releasePromotion); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	releases, err := crane.ListTags(releasePromotion.Spec.ReleasesRepository)
	if err != nil {
		logf.FromContext(ctx).Error(err, "Failed to list tags from releases repository")
		changed := meta.SetStatusCondition(&releasePromotion.Status.Conditions, metav1.Condition{
			Type:               "Available",
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Now(),
			Reason:             "ListTagsFailed",
			Message:            err.Error(),
		})
		if changed {
			r.Status().Update(ctx, &releasePromotion)
		}
		return ctrl.Result{}, err
	}

	if releasePromotion.Spec.Protocol == "oci" {
		err = crane.Copy(
			fmt.Sprintf("%s:%s", releasePromotion.Spec.ReleasesRepository, releases[0]),
			fmt.Sprintf("%s:latest", releasePromotion.Spec.TargetRepository),
		)
		if err != nil {
			logf.FromContext(ctx).Error(err, "Failed to promote artifact from releases repository to target repository")
			changed := meta.SetStatusCondition(&releasePromotion.Status.Conditions, metav1.Condition{
				Type:               "Available",
				Status:             metav1.ConditionFalse,
				LastTransitionTime: metav1.Now(),
				Reason:             "ArtifactPromotionFailed",
				Message:            err.Error(),
			})
			if changed {
				r.Status().Update(ctx, &releasePromotion)
			}
			return ctrl.Result{}, err
		}
	} else {
		// TODO(user): implement s3 protocol
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReleasePromotionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&promotionv1alpha1.ReleasePromotion{}).
		Named("promotion-releasepromotion").
		Complete(r)
}
