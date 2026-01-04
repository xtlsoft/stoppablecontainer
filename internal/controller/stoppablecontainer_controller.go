/*
Copyright 2026.

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
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	scv1alpha1 "github.com/xtlsoft/stoppablecontainer/api/v1alpha1"
)

const (
	// FinalizerName is the finalizer for StoppableContainer
	FinalizerName = "stoppablecontainer.xtlsoft.top/finalizer"

	// ConditionTypeReady indicates the resource is ready
	ConditionTypeReady = "Ready"

	// ConditionTypeProviderReady indicates the provider is ready
	ConditionTypeProviderReady = "ProviderReady"

	// ConditionTypeConsumerReady indicates the consumer is ready
	ConditionTypeConsumerReady = "ConsumerReady"
)

// StoppableContainerReconciler reconciles a StoppableContainer object
type StoppableContainerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=stoppablecontainer.xtlsoft.top,resources=stoppablecontainers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=stoppablecontainer.xtlsoft.top,resources=stoppablecontainers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=stoppablecontainer.xtlsoft.top,resources=stoppablecontainers/finalizers,verbs=update
// +kubebuilder:rbac:groups=stoppablecontainer.xtlsoft.top,resources=stoppablecontainerinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=stoppablecontainer.xtlsoft.top,resources=stoppablecontainerinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile reconciles the StoppableContainer resource
func (r *StoppableContainerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the StoppableContainer
	sc := &scv1alpha1.StoppableContainer{}
	if err := r.Get(ctx, req.NamespacedName, sc); err != nil {
		if errors.IsNotFound(err) {
			log.Info("StoppableContainer not found, ignoring")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !sc.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, sc)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(sc, FinalizerName) {
		controllerutil.AddFinalizer(sc, FinalizerName)
		if err := r.Update(ctx, sc); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Get existing SCI
	sci := &scv1alpha1.StoppableContainerInstance{}
	sciName := types.NamespacedName{
		Namespace: sc.Namespace,
		Name:      sc.Name,
	}
	sciExists := true
	if err := r.Get(ctx, sciName, sci); err != nil {
		if errors.IsNotFound(err) {
			sciExists = false
		} else {
			return ctrl.Result{}, err
		}
	}

	// Reconcile based on desired state
	if sc.Spec.Running {
		// Container should be running
		if !sciExists {
			// Create SCI
			return r.createInstance(ctx, sc)
		}

		// Update SCI if needed
		if !sci.Spec.Running {
			sci.Spec.Running = true
			if err := r.Update(ctx, sci); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("Started container instance")
		}

		// Update status from SCI
		return r.updateStatusFromInstance(ctx, sc, sci)
	} else {
		// Container should be stopped
		if sciExists {
			if sci.Spec.Running {
				// Stop the consumer but keep the provider
				sci.Spec.Running = false
				if err := r.Update(ctx, sci); err != nil {
					return ctrl.Result{}, err
				}
				log.Info("Stopping container instance")
			}
			// Update status from SCI
			return r.updateStatusFromInstance(ctx, sc, sci)
		}

		// No SCI exists and we don't want to run
		return r.updateStatusStopped(ctx, sc)
	}
}

func (r *StoppableContainerReconciler) handleDeletion(ctx context.Context, sc *scv1alpha1.StoppableContainer) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(sc, FinalizerName) {
		return ctrl.Result{}, nil
	}

	// Delete the SCI if it exists
	sci := &scv1alpha1.StoppableContainerInstance{}
	sciName := types.NamespacedName{
		Namespace: sc.Namespace,
		Name:      sc.Name,
	}
	if err := r.Get(ctx, sciName, sci); err == nil {
		// Delete the SCI
		if err := r.Delete(ctx, sci); err != nil && !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		log.Info("Deleted StoppableContainerInstance")
		// Wait for SCI to be fully deleted
		return ctrl.Result{RequeueAfter: time.Second}, nil
	} else if !errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(sc, FinalizerName)
	if err := r.Update(ctx, sc); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("StoppableContainer deleted")
	return ctrl.Result{}, nil
}

func (r *StoppableContainerReconciler) createInstance(ctx context.Context, sc *scv1alpha1.StoppableContainer) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	sci := &scv1alpha1.StoppableContainerInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sc.Name,
			Namespace: sc.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         scv1alpha1.GroupVersion.String(),
					Kind:               "StoppableContainer",
					Name:               sc.Name,
					UID:                sc.UID,
					Controller:         boolPtr(true),
					BlockOwnerDeletion: boolPtr(true),
				},
			},
		},
		Spec: scv1alpha1.StoppableContainerInstanceSpec{
			StoppableContainerName: sc.Name,
			Running:                true,
			Template:               sc.Spec.Template,
			Provider:               sc.Spec.Provider,
			HostPathPrefix:         sc.Spec.HostPathPrefix,
		},
	}

	if err := r.Create(ctx, sci); err != nil {
		if errors.IsAlreadyExists(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	log.Info("Created StoppableContainerInstance")

	// Update status
	sc.Status.InstanceName = sci.Name
	sc.Status.Phase = scv1alpha1.PhasePending
	sc.Status.ObservedGeneration = sc.Generation

	meta.SetStatusCondition(&sc.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeReady,
		Status:             metav1.ConditionFalse,
		Reason:             "InstanceCreated",
		Message:            "StoppableContainerInstance has been created",
		ObservedGeneration: sc.Generation,
	})

	if err := r.Status().Update(ctx, sc); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: time.Second}, nil
}

func (r *StoppableContainerReconciler) updateStatusFromInstance(ctx context.Context, sc *scv1alpha1.StoppableContainer, sci *scv1alpha1.StoppableContainerInstance) (ctrl.Result, error) {
	// Map SCI phase to SC phase
	var phase scv1alpha1.Phase
	var conditionStatus metav1.ConditionStatus
	var reason, message string

	switch sci.Status.Phase {
	case scv1alpha1.InstancePhasePending, scv1alpha1.InstancePhaseProviderStarting:
		phase = scv1alpha1.PhasePending
		conditionStatus = metav1.ConditionFalse
		reason = "Pending"
		message = "Instance is starting up"
	case scv1alpha1.InstancePhaseProviderReady, scv1alpha1.InstancePhaseConsumerStarting:
		phase = scv1alpha1.PhaseProviderReady
		conditionStatus = metav1.ConditionFalse
		reason = "ProviderReady"
		message = "Provider is ready, consumer is starting"
	case scv1alpha1.InstancePhaseRunning:
		phase = scv1alpha1.PhaseRunning
		conditionStatus = metav1.ConditionTrue
		reason = "Running"
		message = "Container is running"
	case scv1alpha1.InstancePhaseStopping, scv1alpha1.InstancePhaseStopped:
		phase = scv1alpha1.PhaseStopped
		conditionStatus = metav1.ConditionFalse
		reason = "Stopped"
		message = "Container is stopped, filesystem preserved"
	case scv1alpha1.InstancePhaseFailed:
		phase = scv1alpha1.PhaseFailed
		conditionStatus = metav1.ConditionFalse
		reason = "Failed"
		message = sci.Status.Message
	default:
		phase = scv1alpha1.PhasePending
		conditionStatus = metav1.ConditionUnknown
		reason = "Unknown"
		message = "Unknown state"
	}

	// Update SC status
	sc.Status.Phase = phase
	sc.Status.InstanceName = sci.Name
	sc.Status.ProviderPodName = sci.Status.ProviderPodName
	sc.Status.ConsumerPodName = sci.Status.ConsumerPodName
	sc.Status.HostPath = sci.Status.HostPath
	sc.Status.NodeName = sci.Status.NodeName
	sc.Status.ObservedGeneration = sc.Generation

	meta.SetStatusCondition(&sc.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeReady,
		Status:             conditionStatus,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: sc.Generation,
	})

	if err := r.Status().Update(ctx, sc); err != nil {
		return ctrl.Result{}, err
	}

	// Requeue to watch for changes
	if phase != scv1alpha1.PhaseRunning && phase != scv1alpha1.PhaseStopped && phase != scv1alpha1.PhaseFailed {
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *StoppableContainerReconciler) updateStatusStopped(ctx context.Context, sc *scv1alpha1.StoppableContainer) (ctrl.Result, error) {
	sc.Status.Phase = scv1alpha1.PhaseStopped
	sc.Status.ObservedGeneration = sc.Generation

	meta.SetStatusCondition(&sc.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeReady,
		Status:             metav1.ConditionFalse,
		Reason:             "Stopped",
		Message:            "Container is stopped, no instance exists",
		ObservedGeneration: sc.Generation,
	})

	if err := r.Status().Update(ctx, sc); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *StoppableContainerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&scv1alpha1.StoppableContainer{}).
		Owns(&scv1alpha1.StoppableContainerInstance{}).
		Watches(
			&scv1alpha1.StoppableContainerInstance{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				sci, ok := obj.(*scv1alpha1.StoppableContainerInstance)
				if !ok {
					return nil
				}
				// Find the parent SC
				return []reconcile.Request{
					{
						NamespacedName: types.NamespacedName{
							Name:      sci.Spec.StoppableContainerName,
							Namespace: sci.Namespace,
						},
					},
				}
			}),
		).
		Named("stoppablecontainer").
		Complete(r)
}

func boolPtr(b bool) *bool {
	return &b
}

func stringPtr(s string) *string {
	return &s
}

func errorf(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}
