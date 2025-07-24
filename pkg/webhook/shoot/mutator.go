// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"fmt"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type mutator struct {
	logger logr.Logger
}

// NewMutator creates a new Mutator that mutates resources in the shoot cluster.
func NewMutator() extensionswebhook.Mutator {
	return &mutator{
		logger: log.Log.WithName("shoot-mutator"),
	}
}

// Mutate mutates resources.
func (m *mutator) Mutate(ctx context.Context, newObj, oldObj client.Object) error {
	newAcc, err := meta.Accessor(newObj)
	if err != nil {
		return fmt.Errorf("could not create accessor during webhook: %w", err)
	}
	// If the object does have a deletion timestamp then we don't want to mutate anything.
	if newAcc.GetDeletionTimestamp() != nil {
		return nil
	}

	switch x := newObj.(type) {
	case *corev1.Node:
		switch y := oldObj.(type) {
		case *corev1.Node:
			return m.mutateNetworkUnavailableNodeCondition(ctx, x, y, func() {
				extensionswebhook.LogMutation(logger, x.Kind, x.Namespace, x.Name)
			})
		}
	}
	return nil
}
