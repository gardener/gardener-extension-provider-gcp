// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"strings"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure/infraflow"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/internal"
	infrainternal "github.com/gardener/gardener-extension-provider-gcp/pkg/internal/infrastructure"
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

func (a *actuator) updateProviderStatus(
	ctx context.Context,
	infra *extensionsv1alpha1.Infrastructure,
	status *v1alpha1.InfrastructureStatus,
	state *runtime.RawExtension,
) error {
	patch := client.MergeFrom(infra.DeepCopy())
	infra.Status.ProviderStatus = &runtime.RawExtension{Object: status}
	infra.Status.State = state
	return a.client.Status().Patch(ctx, infra, patch)
}

func (a *actuator) cleanupTerraformerResources(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure) error {
	tf, err := internal.NewTerraformer(log, a.restConfig, infrainternal.TerraformerPurpose, infra, a.disableProjectedTokenMount)
	if err != nil {
		return err
	}

	if err := tf.CleanupConfiguration(ctx); err != nil {
		return err
	}

	return tf.RemoveTerraformerFinalizerFromConfig(ctx) // Explicitly clean up the terraformer finalizers
}

func getFlowStateFromInfrastructureStatus(infrastructure *extensionsv1alpha1.Infrastructure) (*infraflow.FlowState, error) {
	if infrastructure.Status.State == nil || len(infrastructure.Status.State.Raw) == 0 {
		return nil, nil
	}

	stateJSON, err := infrastructure.Status.State.MarshalJSON()
	if err != nil {
		return nil, err
	}

	isFlowState, err := infraflow.IsJSONFlowState(stateJSON)
	if err != nil {
		return nil, err
	}
	if isFlowState {
		return infraflow.NewFlowStateFromJSON(stateJSON)
	}

	return nil, nil
}

func shouldUseFlow(infra *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) (bool, error) {
	state, err := getFlowStateFromInfrastructureStatus(infra)
	if err != nil {
		return false, err
	}

	if state != nil {
		return true, nil
	}

	return strings.EqualFold(infra.Annotations[gcp.AnnotationKeyUseFlow], "true") ||
		(cluster.Shoot != nil && strings.EqualFold(cluster.Shoot.Annotations[gcp.AnnotationKeyUseFlow], "true")) ||
		(cluster.Seed != nil && strings.EqualFold(cluster.Seed.Labels[gcp.SeedLabelKeyUseFlow], "true")), nil
}
