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
	"time"

	v1alpha1 "github.com/kuberik/propagation-controller/api/v1alpha1"
	"github.com/kuberik/propagation-controller/pkg/clients"
	"github.com/kuberik/propagation-controller/pkg/repo/config"
	"golang.org/x/sync/errgroup"
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

type PropagationReadyReason string

const (
	BackendInitFailedPropagationReadyReason PropagationReadyReason = "BackendInitFailed"
	ConfigInitFailedPropagationReadyReason  PropagationReadyReason = "ConfigInitFailed"
	VersionMissingPropagationReadyReason    PropagationReadyReason = "VersionMissing"
	ReadyPropagationReadyReason             PropagationReadyReason = "Ready"
)

// PropagationReconciler reconciles a Propagation object
type PropagationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	clients.PropagationClientset
}

//+kubebuilder:rbac:groups=kuberik.io,resources=propagations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kuberik.io,resources=propagations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kuberik.io,resources=propagations/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PropagationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName(req.NamespacedName.String())

	propagation := &v1alpha1.Propagation{}
	err := r.Client.Get(ctx, req.NamespacedName, propagation)
	if err != nil {
		return ctrl.Result{}, err
	}

	propagationClient, err := r.Propagation(*propagation)
	if err != nil {
		return r.SetReadyConditionFalse(ctx, propagation, err, BackendInitFailedPropagationReadyReason)
	}

	config, err := propagationClient.GetConfig()
	if err != nil {
		return r.SetReadyConditionFalse(ctx, propagation, err, ConfigInitFailedPropagationReadyReason)
	}

	deployConditions, err := deployConditionsFromConfig(*propagation, *config)
	if err != nil {
		return r.SetReadyConditionFalse(ctx, propagation, err, ConfigInitFailedPropagationReadyReason)
	}
	propagation.Status.DeployConditions = *deployConditions
	if err := r.Client.Status().Update(ctx, propagation); err != nil {
		return ctrl.Result{}, err
	}

	version, err := r.getVersion(ctx, *propagation)
	if err != nil {
		return r.SetReadyConditionFalse(ctx, propagation, err, VersionMissingPropagationReadyReason)
	}

	if readyCondition := meta.FindStatusCondition(
		propagation.Status.Conditions, v1alpha1.ReadyCondition,
	); readyCondition == nil || readyCondition.Status != metav1.ConditionTrue {
		meta.SetStatusCondition(&propagation.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.ReadyCondition,
			Status:             metav1.ConditionTrue,
			Message:            "Propagation ready",
			ObservedGeneration: propagation.Generation,
			Reason:             string(ReadyPropagationReadyReason),
		})
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

		if err := propagationClient.PublishStatus(
			propagation.Spec.Deployment.Name,
			propagation.Status.DeploymentStatus,
		); err != nil {
			return ctrl.Result{}, err
		}

		err = r.Client.Status().Update(ctx, propagation)
		if err != nil {
			return ctrl.Result{}, nil
		}
	}

	// if status healthy, propagate
	// 1. fetch statuses
	var getStatusErrors errgroup.Group
	requiredStatuses := append(propagation.Status.DeployConditions.DeployWith, propagation.Status.DeployConditions.DeployAfter.Deployments...)
	reports := make([]v1alpha1.DeploymentStatusesReport, len(requiredStatuses))
	for i, deployment := range requiredStatuses {
		report := propagation.Status.FindDeploymentStatusReport(deployment)
		if report == nil {
			report = &v1alpha1.DeploymentStatusesReport{
				DeploymentName: deployment,
			}
		}
		reports[i] = *report
		i, deployment := i, deployment
		getStatusErrors.Go(func() error {
			status, err := propagationClient.GetStatus(deployment)
			if err != nil {
				return err
			}
			status.Start = metav1.Now()
			reports[i].AppendStatus(*status)
			return nil
		})
	}
	statusesErr := getStatusErrors.Wait()
	if statusesErr != nil {
		meta.SetStatusCondition(&propagation.Status.Conditions, metav1.Condition{
			Type:               "StatusesFetched",
			Status:             metav1.ConditionFalse,
			Message:            fmt.Sprintf("Error fetching statuses: %v", statusesErr),
			ObservedGeneration: propagation.Generation,
			Reason:             "StatusesFetchFailed",
		})
		r.Client.Status().Update(ctx, propagation)
		return ctrl.Result{}, statusesErr
	}
	meta.SetStatusCondition(&propagation.Status.Conditions, metav1.Condition{
		Type:               "StatusesFetched",
		Status:             metav1.ConditionTrue,
		Message:            "Statuses fetched",
		ObservedGeneration: propagation.Generation,
		Reason:             "StatusesFetchSuccess",
	})
	propagation.Status.DeploymentStatusesReports = reports
	if err := r.Client.Status().Update(ctx, propagation); err != nil {
		return ctrl.Result{}, err
	}

	waitTime := propagation.Spec.PollInterval.Duration
	if waitTime == 0 {
		waitTime = time.Minute
	}

	// check if all deployWith are on the same version as current deployment
	for _, deployment := range propagation.Status.DeployConditions.DeployWith {
		deploymentStatus := propagation.Status.FindDeploymentStatusReport(deployment)
		if deploymentStatus == nil ||
			deploymentStatus.LastStatus().Version != version ||
			deploymentStatus.LastStatus().State != v1alpha1.HealthStateHealthy ||
			deploymentStatus.VersionHealthyDuration(version) < config.DeploymentBakeTime(deployment) {
			log.Info(fmt.Sprintf("Waiting for deployment %s to bake", deployment))
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: waitTime,
			}, nil
		}
	}

	propagateVersion := propagation.NextVersion()
	if propagateVersion == "" {
		// TODO: Calculate from status how long to wait
		log.Info("No candidate version to propagate")
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: waitTime,
		}, nil
	}
	log.Info(fmt.Sprintf("Propagating to version %s", propagateVersion))
	if err := propagationClient.Propagate(propagation.Spec.Deployment.Name, propagateVersion); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: waitTime,
	}, nil
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

func (r *PropagationReconciler) SetReadyConditionFalse(ctx context.Context, propagation *v1alpha1.Propagation, err error, reason PropagationReadyReason) (ctrl.Result, error) {
	meta.SetStatusCondition(&propagation.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.ReadyCondition,
		Message:            err.Error(),
		Status:             metav1.ConditionFalse,
		ObservedGeneration: propagation.Generation,
		Reason:             string(reason),
	})
	r.Client.Status().Update(ctx, propagation)
	return ctrl.Result{}, err
}

// deployAfterFromConfig determines based on the global deployment config which deployments precede the current one in the propagation sequence.
// It identifies the the wave and environment of the current deployment and returns the deployments from the previous wave.
func deployAfterFromConfig(propagation v1alpha1.Propagation, c config.Config) (*v1alpha1.DeployAfter, error) {
	var lastWave config.Wave
	for _, env := range c.Environments {
		for _, wave := range env.Waves {
			for _, deployment := range wave.Deployments {
				if deployment == propagation.Spec.Deployment.Name {
					return &v1alpha1.DeployAfter{
						Deployments: lastWave.Deployments,
						BakeTime:    lastWave.BakeTime,
					}, nil
				}
			}
			lastWave = wave
		}
	}
	return nil, fmt.Errorf("failed to find the config for '%s' deployment", propagation.Spec.Deployment.Name)
}

// deployWithFromConfig deploys determines which deployments needs to have same version as the current deployment before proceeding to propagate to next version.
// The returned list of deployments are all deployments from all subsequent waves in the same environment.
func deployWithFromConfig(propagation v1alpha1.Propagation, c config.Config) ([]string, error) {
	deployments := []string{}
	for _, env := range c.Environments {
		for waveIdx, wave := range env.Waves {
			if slices.Contains(wave.Deployments, propagation.Spec.Deployment.Name) {
				if waveIdx == 0 {
					for _, wave := range env.Waves[waveIdx+1:] {
						deployments = append(deployments, wave.Deployments...)
					}
					return deployments, nil
				}
				return []string{}, nil
			}
		}
	}
	return nil, fmt.Errorf("failed to find the config for '%s' deployment", propagation.Spec.Deployment.Name)
}

func deployConditionsFromConfig(propagation v1alpha1.Propagation, c config.Config) (*v1alpha1.DeployConditions, error) {
	deployAfter, err := deployAfterFromConfig(propagation, c)
	if err != nil {
		return nil, err
	}
	deployWith, err := deployWithFromConfig(propagation, c)
	if err != nil {
		return nil, err
	}
	return &v1alpha1.DeployConditions{
		DeployAfter: *deployAfter,
		DeployWith:  deployWith,
	}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PropagationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Propagation{}).
		Complete(r)
}
