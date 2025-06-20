// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shootservice

import (
	"context"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Mutator", func() {
	var (
		fakeShootClient            = fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()
		loadBalancerServiceMapMeta = metav1.ObjectMeta{Name: "externalLoadbalancer", Namespace: metav1.NamespaceSystem}
		ctxWithClient              = context.Background()
	)

	Expect(fakeShootClient.Create(context.TODO(), &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-dns", Namespace: "kube-system"},
		Spec: corev1.ServiceSpec{
			IPFamilies: []corev1.IPFamily{corev1.IPv6Protocol},
		},
	})).To(Succeed())
	ctxWithClient = context.WithValue(ctxWithClient, extensionswebhook.ShootClientContextKey{}, fakeShootClient)
	mutator := &mutator{wantsShootClient: false} // we use the fake client from the ctx

	DescribeTable("#Mutate",
		func(service *corev1.Service) {
			service.Annotations = make(map[string]string, 1)
			err := mutator.Mutate(ctxWithClient, service, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(service.Annotations).To(HaveKeyWithValue("cloud.google.com/l4-rbs", "enabled"))
		},

		Entry("IPv6-only", &corev1.Service{ObjectMeta: loadBalancerServiceMapMeta, Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer, IPFamilies: []corev1.IPFamily{corev1.IPv6Protocol}}}),
		Entry("dual-stack", &corev1.Service{ObjectMeta: loadBalancerServiceMapMeta, Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer, IPFamilies: []corev1.IPFamily{corev1.IPv6Protocol, corev1.IPv4Protocol}}}),
	)

	DescribeTable("#Mutate",
		func(service *corev1.Service) {
			Expect(fakeShootClient.Patch(context.TODO(), &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "kube-dns", Namespace: "kube-system"},
				Spec: corev1.ServiceSpec{
					IPFamilies: []corev1.IPFamily{corev1.IPv4Protocol},
				},
			}, client.MergeFrom(&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "kube-dns", Namespace: "kube-system"}}))).To(Succeed())
			service.Annotations = make(map[string]string, 1)
			err := mutator.Mutate(ctxWithClient, service, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(service.Annotations).ToNot(HaveKeyWithValue("cloud.google.com/l4-rbs", "enabled"))
		},
		Entry("IPv4-only", &corev1.Service{ObjectMeta: loadBalancerServiceMapMeta, Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer, IPFamilies: []corev1.IPFamily{corev1.IPv4Protocol}}}),
	)

	DescribeTable("#Mutate",
		func(service *corev1.Service) {
			metav1.SetMetaDataAnnotation(&service.ObjectMeta, "networking.gke.io/load-balancer-type", "Internal")
			Expect(fakeShootClient.Patch(context.TODO(), &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "kube-dns", Namespace: "kube-system"},
				Spec: corev1.ServiceSpec{
					IPFamilies: []corev1.IPFamily{corev1.IPv4Protocol},
				},
			}, client.MergeFrom(&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "kube-dns", Namespace: "kube-system"}}))).To(Succeed())
			err := mutator.Mutate(ctxWithClient, service, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(service.Annotations).ToNot(HaveKeyWithValue("cloud.google.com/l4-rbs", "enabled"))
		},

		Entry("dual-stack", &corev1.Service{ObjectMeta: loadBalancerServiceMapMeta, Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer, IPFamilies: []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol}}}),
	)

	DescribeTable("#Mutate",
		func(service *corev1.Service) {
			metav1.SetMetaDataAnnotation(&service.ObjectMeta, "cloud.google.com/load-balancer-type", "internal")
			Expect(fakeShootClient.Patch(context.TODO(), &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "kube-dns", Namespace: "kube-system"},
				Spec: corev1.ServiceSpec{
					IPFamilies: []corev1.IPFamily{corev1.IPv4Protocol},
				},
			}, client.MergeFrom(&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "kube-dns", Namespace: "kube-system"}}))).To(Succeed())
			err := mutator.Mutate(ctxWithClient, service, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(service.Annotations).ToNot(HaveKeyWithValue("cloud.google.com/l4-rbs", "enabled"))
		},

		Entry("dual-stack", &corev1.Service{ObjectMeta: loadBalancerServiceMapMeta, Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer, IPFamilies: []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol}}}),
	)

	It("should return error if resource is not a Service", func() {
		err := mutator.Mutate(ctxWithClient, &corev1.ConfigMap{}, nil)
		Expect(err).To(HaveOccurred())
	})

	It("should return nil if Service is not a LoadBalancer", func() {
		service := &corev1.Service{ObjectMeta: loadBalancerServiceMapMeta, Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, IPFamilies: []corev1.IPFamily{corev1.IPv6Protocol}}}
		err := mutator.Mutate(ctxWithClient, service, nil)
		Expect(err).To(Not(HaveOccurred()))
	})
})
