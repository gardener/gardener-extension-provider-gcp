// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
)

type actuator struct {
	client                     client.Client
	restConfig                 *rest.Config
	disableProjectedTokenMount bool
}

// NewActuator creates a new infrastructure.Actuator.
func NewActuator(mgr manager.Manager, disableProjectedTokenMount bool) infrastructure.Actuator {
	return &actuator{
		client:                     mgr.GetClient(),
		restConfig:                 mgr.GetConfig(),
		disableProjectedTokenMount: disableProjectedTokenMount,
	}
}

// CleanupTerraformerResources deletes terraformer artifacts (config, state, secrets).
func CleanupTerraformerResources(ctx context.Context, tf terraformer.Terraformer) error {
	if err := tf.EnsureCleanedUp(ctx); err != nil {
		return err
	}
	if err := tf.CleanupConfiguration(ctx); err != nil {
		return err
	}
	return tf.RemoveTerraformerFinalizerFromConfig(ctx)
}

func patchProviderStatusAndState(
	ctx context.Context,
	runtimeClient client.Client,
	infra *extensionsv1alpha1.Infrastructure,
	status *v1alpha1.InfrastructureStatus,
	state *runtime.RawExtension,
) error {
	patch := client.MergeFrom(infra.DeepCopy())
	if status != nil {
		infra.Status.ProviderStatus = &runtime.RawExtension{Object: status}
		for _, natIP := range status.Networks.NatIPs {
			infra.Status.EgressCIDRs = append(infra.Status.EgressCIDRs, fmt.Sprintf("%s/32", natIP.IP))
		}
	}
	if state != nil {
		infra.Status.State = state
	}

	if data, err := patch.Data(infra); err != nil {
		return fmt.Errorf("failed getting patch data for infra %s: %w", infra.Name, err)
	} else if string(data) == `{}` {
		return nil
	}

	return runtimeClient.Status().Patch(ctx, infra, patch)
}
