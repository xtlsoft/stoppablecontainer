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
)

var _ = Describe("StoppableContainer Controller", func() {
	Context("When reconciling a resource", func() {
		It("should add finalizer to the resource", func() {
			ctx := context.Background()
			resourceName := "test-sc-finalizer"

			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			By("Creating the StoppableContainer resource")
			resource := &scv1alpha1.StoppableContainer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: scv1alpha1.StoppableContainerSpec{
					Running: false,
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
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			By("Reconciling the created resource")
			controllerReconciler := &StoppableContainerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify finalizer was added
			sc := &scv1alpha1.StoppableContainer{}
			err = k8sClient.Get(ctx, typeNamespacedName, sc)
			Expect(err).NotTo(HaveOccurred())
			Expect(sc.Finalizers).To(ContainElement(FinalizerName))

			// Cleanup
			Expect(k8sClient.Delete(ctx, sc)).To(Succeed())
		})
	})
})
