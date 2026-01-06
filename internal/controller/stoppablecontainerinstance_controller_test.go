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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	scv1alpha1 "github.com/xtlsoft/stoppablecontainer/api/v1alpha1"
	"github.com/xtlsoft/stoppablecontainer/internal/provider"
)

var _ = Describe("StoppableContainerInstance Controller", func() {
	Context("When reconciling a resource", func() {
		It("should add finalizer and create provider pod", func() {
			ctx := context.Background()
			resourceName := "test-sci-provider"

			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			By("Creating the StoppableContainerInstance resource")
			resource := &scv1alpha1.StoppableContainerInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: scv1alpha1.StoppableContainerInstanceSpec{
					StoppableContainerName: resourceName,
					Running:                true,
					Template: scv1alpha1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:    "main",
									Image:   "ubuntu:22.04",
									Command: []string{"sleep", "infinity"},
								},
							},
						},
					},
					HostPathPrefix: "/var/lib/stoppablecontainer",
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			By("Reconciling the created resource")
			controllerReconciler := &StoppableContainerInstanceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			// First reconcile adds finalizer
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify finalizer was added
			sci := &scv1alpha1.StoppableContainerInstance{}
			err = k8sClient.Get(ctx, typeNamespacedName, sci)
			Expect(err).NotTo(HaveOccurred())
			Expect(sci.Finalizers).To(ContainElement(SCIFinalizerName))

			// Second reconcile creates provider pod
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify provider pod was created
			providerPod := &corev1.Pod{}
			providerPodName := types.NamespacedName{
				Namespace: "default",
				Name:      resourceName + "-provider",
			}
			err = k8sClient.Get(ctx, providerPodName, providerPod)
			Expect(err).NotTo(HaveOccurred())
			Expect(providerPod.Labels[provider.LabelRole]).To(Equal("provider"))
			Expect(providerPod.Labels[provider.LabelInstance]).To(Equal(resourceName))

			// Cleanup
			Expect(k8sClient.Delete(ctx, providerPod)).To(Succeed())
			Expect(k8sClient.Delete(ctx, sci)).To(Succeed())
		})
	})
})
