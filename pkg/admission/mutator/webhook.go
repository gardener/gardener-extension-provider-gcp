// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mutator

import (
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

const (
	// Name is a name for a validation webhook.
	Name = "mutator"
)

var logger = log.Log.WithName("gcp-mutator-webhook")

// New creates a new webhook that mutates Shoot and NamespacedCloudProfile resources.
func New(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	logger.Info("Setting up webhook", "name", Name)

	return extensionswebhook.New(mgr, extensionswebhook.Args{
		Provider: gcp.Type,
		Name:     Name,
		Path:     "/webhooks/mutate",
		Mutators: map[extensionswebhook.Mutator][]extensionswebhook.Type{
			NewShootMutator(mgr):                  {{Obj: &gardencorev1beta1.Shoot{}}},
			NewNamespacedCloudProfileMutator(mgr): {{Obj: &gardencorev1beta1.NamespacedCloudProfile{}, Subresource: ptr.To("status")}},
		},
		Target: extensionswebhook.TargetSeed,
		ObjectSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"provider.extensions.gardener.cloud/gcp": "true"},
		},
	})
}
