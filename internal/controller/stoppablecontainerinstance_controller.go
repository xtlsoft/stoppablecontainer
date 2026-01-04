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
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
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
	"github.com/xtlsoft/stoppablecontainer/internal/provider"
)

const (
	// SCIFinalizerName is the finalizer for StoppableContainerInstance
	SCIFinalizerName = "stoppablecontainerinstance.xtlsoft.top/finalizer"
)

// StoppableContainerInstanceReconciler reconciles a StoppableContainerInstance object
type StoppableContainerInstanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=stoppablecontainer.xtlsoft.top,resources=stoppablecontainerinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=stoppablecontainer.xtlsoft.top,resources=stoppablecontainerinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=stoppablecontainer.xtlsoft.top,resources=stoppablecontainerinstances/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile reconciles the StoppableContainerInstance resource
func (r *StoppableContainerInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the SCI
	sci := &scv1alpha1.StoppableContainerInstance{}
	if err := r.Get(ctx, req.NamespacedName, sci); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !sci.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, sci)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(sci, SCIFinalizerName) {
		controllerutil.AddFinalizer(sci, SCIFinalizerName)
		if err := r.Update(ctx, sci); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Get provider pod
	providerPod := &corev1.Pod{}
	providerPodName := types.NamespacedName{
		Namespace: sci.Namespace,
		Name:      fmt.Sprintf("%s-provider", sci.Name),
	}
	providerExists := true
	if err := r.Get(ctx, providerPodName, providerPod); err != nil {
		if errors.IsNotFound(err) {
			providerExists = false
		} else {
			return ctrl.Result{}, err
		}
	}

	// Get consumer pod
	consumerPod := &corev1.Pod{}
	consumerPodName := types.NamespacedName{
		Namespace: sci.Namespace,
		Name:      fmt.Sprintf("%s-consumer", sci.Name),
	}
	consumerExists := true
	if err := r.Get(ctx, consumerPodName, consumerPod); err != nil {
		if errors.IsNotFound(err) {
			consumerExists = false
		} else {
			return ctrl.Result{}, err
		}
	}

	// Reconcile provider pod
	if !providerExists {
		return r.createProviderPod(ctx, sci)
	}

	// Check provider pod status
	if !isPodReady(providerPod) {
		return r.updatePhase(ctx, sci, scv1alpha1.InstancePhaseProviderStarting,
			"Waiting for provider pod to be ready")
	}

	// Provider is ready - update node name and host path
	sci.Status.NodeName = providerPod.Spec.NodeName
	sci.Status.HostPath = filepath.Join(provider.GetHostPath(sci), "rootfs")
	sci.Status.ProviderPodName = providerPod.Name
	sci.Status.ProviderPodUID = string(providerPod.UID)

	// If we shouldn't be running, make sure consumer is deleted
	if !sci.Spec.Running {
		if consumerExists {
			log.Info("Deleting consumer pod (stopping)")
			if err := r.Delete(ctx, consumerPod); err != nil && !errors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
			return r.updatePhase(ctx, sci, scv1alpha1.InstancePhaseStopping,
				"Stopping consumer pod")
		}
		return r.updatePhase(ctx, sci, scv1alpha1.InstancePhaseStopped,
			"Consumer stopped, provider maintaining filesystem")
	}

	// We should be running - create consumer if needed
	if !consumerExists {
		// Make sure provider is fully ready first
		if sci.Status.NodeName == "" {
			return r.updatePhase(ctx, sci, scv1alpha1.InstancePhaseProviderReady,
				"Provider ready, waiting for node assignment")
		}
		return r.createConsumerPod(ctx, sci)
	}

	// Check consumer pod status
	sci.Status.ConsumerPodName = consumerPod.Name
	sci.Status.ConsumerPodUID = string(consumerPod.UID)

	if isPodFailed(consumerPod) {
		return r.updatePhase(ctx, sci, scv1alpha1.InstancePhaseFailed,
			fmt.Sprintf("Consumer pod failed: %s", getPodFailureReason(consumerPod)))
	}

	if !isPodReady(consumerPod) {
		return r.updatePhase(ctx, sci, scv1alpha1.InstancePhaseConsumerStarting,
			"Waiting for consumer pod to be ready")
	}

	// Everything is running
	return r.updatePhase(ctx, sci, scv1alpha1.InstancePhaseRunning,
		"All pods running")
}

func (r *StoppableContainerInstanceReconciler) handleDeletion(ctx context.Context, sci *scv1alpha1.StoppableContainerInstance) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(sci, SCIFinalizerName) {
		return ctrl.Result{}, nil
	}

	// Delete consumer pod if exists
	consumerPod := &corev1.Pod{}
	consumerPodName := types.NamespacedName{
		Namespace: sci.Namespace,
		Name:      fmt.Sprintf("%s-consumer", sci.Name),
	}
	if err := r.Get(ctx, consumerPodName, consumerPod); err == nil {
		if err := r.Delete(ctx, consumerPod); err != nil && !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		log.Info("Deleted consumer pod")
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	// Delete provider pod if exists
	providerPod := &corev1.Pod{}
	providerPodName := types.NamespacedName{
		Namespace: sci.Namespace,
		Name:      fmt.Sprintf("%s-provider", sci.Name),
	}
	if err := r.Get(ctx, providerPodName, providerPod); err == nil {
		if err := r.Delete(ctx, providerPod); err != nil && !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		log.Info("Deleted provider pod")
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(sci, SCIFinalizerName)
	if err := r.Update(ctx, sci); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("StoppableContainerInstance deleted")
	return ctrl.Result{}, nil
}

func (r *StoppableContainerInstanceReconciler) createProviderPod(ctx context.Context, sci *scv1alpha1.StoppableContainerInstance) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	builder := provider.NewProviderPodBuilder(sci)
	pod := builder.Build()

	if err := r.Create(ctx, pod); err != nil {
		if errors.IsAlreadyExists(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	log.Info("Created provider pod", "name", pod.Name)
	return r.updatePhase(ctx, sci, scv1alpha1.InstancePhaseProviderStarting,
		"Provider pod created")
}

func (r *StoppableContainerInstanceReconciler) createConsumerPod(ctx context.Context, sci *scv1alpha1.StoppableContainerInstance) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if sci.Status.NodeName == "" {
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	builder := provider.NewConsumerPodBuilder(sci, sci.Status.NodeName)
	pod := builder.Build()

	if err := r.Create(ctx, pod); err != nil {
		if errors.IsAlreadyExists(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	log.Info("Created consumer pod", "name", pod.Name, "node", sci.Status.NodeName)
	return r.updatePhase(ctx, sci, scv1alpha1.InstancePhaseConsumerStarting,
		"Consumer pod created")
}

func (r *StoppableContainerInstanceReconciler) updatePhase(ctx context.Context, sci *scv1alpha1.StoppableContainerInstance, phase scv1alpha1.InstancePhase, message string) (ctrl.Result, error) {
	sci.Status.Phase = phase
	sci.Status.Message = message
	sci.Status.ObservedGeneration = sci.Generation

	var conditionStatus metav1.ConditionStatus
	var reason string

	switch phase {
	case scv1alpha1.InstancePhaseRunning:
		conditionStatus = metav1.ConditionTrue
		reason = "Running"
	case scv1alpha1.InstancePhaseFailed:
		conditionStatus = metav1.ConditionFalse
		reason = "Failed"
	default:
		conditionStatus = metav1.ConditionFalse
		reason = string(phase)
	}

	meta.SetStatusCondition(&sci.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeReady,
		Status:             conditionStatus,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: sci.Generation,
	})

	if err := r.Status().Update(ctx, sci); err != nil {
		return ctrl.Result{}, err
	}

	// Requeue for intermediate states
	if phase != scv1alpha1.InstancePhaseRunning && phase != scv1alpha1.InstancePhaseStopped && phase != scv1alpha1.InstancePhaseFailed {
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func isPodReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func isPodFailed(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodFailed
}

func getPodFailureReason(pod *corev1.Pod) string {
	if pod.Status.Message != "" {
		return pod.Status.Message
	}
	if pod.Status.Reason != "" {
		return pod.Status.Reason
	}
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
			return cs.State.Waiting.Reason
		}
		if cs.State.Terminated != nil && cs.State.Terminated.Reason != "" {
			return cs.State.Terminated.Reason
		}
	}
	return "Unknown"
}

// SetupWithManager sets up the controller with the Manager.
func (r *StoppableContainerInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&scv1alpha1.StoppableContainerInstance{}).
		Owns(&corev1.Pod{}).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				pod, ok := obj.(*corev1.Pod)
				if !ok {
					return nil
				}

				// Check if this pod is managed by us
				if pod.Labels == nil {
					return nil
				}

				instance, ok := pod.Labels[provider.LabelInstance]
				if !ok {
					return nil
				}

				managedBy, ok := pod.Labels[provider.LabelManagedBy]
				if !ok || managedBy != "stoppablecontainer" {
					return nil
				}

				return []reconcile.Request{
					{
						NamespacedName: types.NamespacedName{
							Name:      instance,
							Namespace: pod.Namespace,
						},
					},
				}
			}),
		).
		Named("stoppablecontainerinstance").
		Complete(r)
}
