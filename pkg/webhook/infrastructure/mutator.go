// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	extensionscontextwebhook "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

type mutator struct {
	logger logr.Logger
	client client.Client
}

// New returns a new Infrastructure mutator that uses mutateFunc to perform the mutation.
func New(mgr manager.Manager, logger logr.Logger) extensionswebhook.Mutator {
	return &mutator{
		client: mgr.GetClient(),
		logger: logger,
	}
}

// Mutate mutates the given object on creation and adds the annotation `gcp.provider.extensions.gardener.cloud/use-flow=true`
// if the seed has the label `gcp.provider.extensions.gardener.cloud/use-flow` == `new`.
func (m *mutator) Mutate(ctx context.Context, new, old client.Object) error {
	if old != nil || new.GetDeletionTimestamp() != nil {
		return nil
	}

	newInfra, ok := new.(*extensionsv1alpha1.Infrastructure)
	if !ok {
		return fmt.Errorf("could not mutate: object is not of type Infrastructure")
	}

	if m.isInMigrationOrRestorePhase(newInfra) {
		return nil
	}

	gctx := extensionscontextwebhook.NewGardenContext(m.client, new)
	cluster, err := gctx.GetCluster(ctx)
	if err != nil {
		return err
	}

	if cluster.Seed.Labels[gcp.SeedLabelKeyUseFlow] == gcp.SeedLabelUseFlowValueNew {
		if newInfra.Annotations == nil {
			newInfra.Annotations = map[string]string{}
		}
		newInfra.Annotations[gcp.AnnotationKeyUseFlow] = "true"
	}

	return nil
}

func (m *mutator) isInMigrationOrRestorePhase(infra *extensionsv1alpha1.Infrastructure) bool {
	operationType := v1beta1helper.ComputeOperationType(infra.ObjectMeta, infra.Status.LastOperation)

	return operationType == v1beta1.LastOperationTypeMigrate || operationType == v1beta1.LastOperationTypeRestore
}
