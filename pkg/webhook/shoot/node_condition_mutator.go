// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"

	corev1 "k8s.io/api/core/v1"
)

const (
	noRouteCreated          = "NoRouteCreated"
	nodeCreatedWithoutRoute = "Node created without a route"
)

func (m *mutator) mutateNetworkUnavailableNodeCondition(_ context.Context, new *corev1.Node, old *corev1.Node, logMutation func()) error {
	if new == nil || old == nil {
		return nil
	}
	for i, c := range new.Status.Conditions {
		if c.Type == corev1.NodeNetworkUnavailable && c.Status == corev1.ConditionTrue && c.Reason == noRouteCreated && c.Message == nodeCreatedWithoutRoute {
			logMutation()
			for _, oldCondition := range old.Status.Conditions {
				if oldCondition.Type == corev1.NodeNetworkUnavailable && oldCondition.Status == corev1.ConditionFalse {
					new.Status.Conditions[i] = oldCondition
					return nil
				}
			}
			// Did not find the condition in the old object => remove the condition
			new.Status.Conditions = append(new.Status.Conditions[:i], new.Status.Conditions[i+1:]...)
			return nil
		}
	}

	return nil
}
