// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shootservice

import (
	"context"
	"fmt"
	"slices"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mutator struct {
	logger logr.Logger
}

// NewMutatorWithShootClient creates a new Mutator that mutates resources in the shoot cluster.
func NewMutatorWithShootClient(logger logr.Logger) extensionswebhook.MutatorWithShootClient {
	return &mutator{logger}
}

// Mutate mutates resources.
func (m *mutator) Mutate(ctx context.Context, new, _ client.Object, shootClient client.Client) error {
	service, ok := new.(*corev1.Service)
	if !ok {
		return fmt.Errorf("could not mutate: object is not of type corev1.Service")
	}

	// If the object does have a deletion timestamp then we don't want to mutate anything.
	if service.GetDeletionTimestamp() != nil {
		return nil
	}
	extensionswebhook.LogMutation(m.logger, service.Kind, service.Namespace, service.Name)

	if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return nil
	}

	if metav1.HasAnnotation(service.ObjectMeta, "networking.gke.io/load-balancer-type") &&
		(service.Annotations["networking.gke.io/load-balancer-type"] == "Internal" ||
			service.Annotations["networking.gke.io/load-balancer-type"] == "internal") ||
		metav1.HasAnnotation(service.ObjectMeta, "cloud.google.com/load-balancer-type") &&
			(service.Annotations["cloud.google.com/load-balancer-type"] == "Internal" ||
				service.Annotations["cloud.google.com/load-balancer-type"] == "internal") {
		return nil
	}

	kubeDNSService := &corev1.Service{}
	if err := shootClient.Get(ctx, types.NamespacedName{Name: "kube-dns", Namespace: "kube-system"}, kubeDNSService); err != nil {
		return err
	}
	if slices.Contains(kubeDNSService.Spec.IPFamilies, corev1.IPv6Protocol) {
		metav1.SetMetaDataAnnotation(&service.ObjectMeta, "cloud.google.com/l4-rbs", "enabled")
	}

	return nil
}
