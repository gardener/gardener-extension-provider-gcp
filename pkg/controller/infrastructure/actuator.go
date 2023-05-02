// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package infrastructure

import (
	"context"
	"strings"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/common"
	"github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure/infraflow"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/internal"
	infrainternal "github.com/gardener/gardener-extension-provider-gcp/pkg/internal/infrastructure"
)

type actuator struct {
	common.RESTConfigContext
	disableProjectedTokenMount bool
}

// NewActuator creates a new infrastructure.Actuator.
func NewActuator(disableProjectedTokenMount bool) infrastructure.Actuator {
	return &actuator{
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
	return a.Client().Status().Patch(ctx, infra, patch)
}

func (a *actuator) cleanupTerraformerResources(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure) error {
	tf, err := internal.NewTerraformer(log, a.RESTConfig(), infrainternal.TerraformerPurpose, infra, a.disableProjectedTokenMount)
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
