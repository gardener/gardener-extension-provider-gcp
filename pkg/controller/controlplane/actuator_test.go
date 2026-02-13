// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	genericcontrolplaneactuator "github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

var _ = Describe("Actuator", func() {
	var (
		ctx    context.Context
		lg     logr.Logger
		scheme *runtime.Scheme
	)

	BeforeEach(func() {
		ctx = context.TODO()
		lg = logr.Discard()

		scheme = runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(resourcesv1alpha1.AddToScheme(scheme)).To(Succeed())
	})

	Describe("#Reconcile", func() {
		Context("removeIgnoreAnnotations", func() {
			It("should not modify annotations if remove-ignore is absent", func() {
				clusterName := "shoot--foo--bar"
				mr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      genericcontrolplaneactuator.ControlPlaneSeedChartResourceName,
						Namespace: clusterName,
						Annotations: map[string]string{
							resourcesv1alpha1.Ignore: "true",
						},
					},
				}
				c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(mr).Build()
				a := &actuator{client: c}
				cluster := &extensionscontroller.Cluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName}}
				Expect(a.removeIgnoreAnnotations(ctx, lg, cluster)).To(Succeed())

				got := &resourcesv1alpha1.ManagedResource{}
				Expect(c.Get(ctx, ctrlclient.ObjectKey{Namespace: clusterName,
					Name: genericcontrolplaneactuator.ControlPlaneSeedChartResourceName}, got)).To(Succeed())
				Expect(got.Annotations).ToNot(HaveKey(gcp.AnnotationRemoveIgnore))
				Expect(got.Annotations).To(HaveKey(resourcesv1alpha1.Ignore))
			})

			It("should remove both annotations when remove-ignore is present", func() {
				clusterName := "shoot--foo--bar"
				mr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      genericcontrolplaneactuator.ControlPlaneSeedChartResourceName,
						Namespace: clusterName,
						Annotations: map[string]string{
							gcp.AnnotationRemoveIgnore: "true",
							resourcesv1alpha1.Ignore:   "true",
						},
					},
				}
				c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(mr).Build()
				a := &actuator{client: c}
				cluster := &extensionscontroller.Cluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName}}
				Expect(a.removeIgnoreAnnotations(ctx, lg, cluster)).To(Succeed())

				got := &resourcesv1alpha1.ManagedResource{}
				Expect(c.Get(ctx, ctrlclient.ObjectKey{Namespace: clusterName,
					Name: genericcontrolplaneactuator.ControlPlaneSeedChartResourceName}, got)).To(Succeed())
				Expect(got.Annotations).ToNot(HaveKey(gcp.AnnotationRemoveIgnore))
				Expect(got.Annotations).ToNot(HaveKey(resourcesv1alpha1.Ignore))
			})
		})
	})
})
