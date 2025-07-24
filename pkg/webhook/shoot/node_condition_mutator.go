// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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

func (m *mutator) mutateNetworkUnavailableNodeCondition(
	_ context.Context,
	newNode *corev1.Node,
	oldNode *corev1.Node,
	logMutation func()) error {
	if newNode == nil || oldNode == nil {
		return nil
	}
	for i, c := range newNode.Status.Conditions {
		if c.Type == corev1.NodeNetworkUnavailable && c.Status == corev1.ConditionTrue && c.Reason == noRouteCreated && c.Message == nodeCreatedWithoutRoute {
			logMutation()
			for _, oldCondition := range oldNode.Status.Conditions {
				if oldCondition.Type == corev1.NodeNetworkUnavailable && oldCondition.Status == corev1.ConditionFalse {
					newNode.Status.Conditions[i] = oldCondition
					return nil
				}
			}
			// Did not find the condition in the old object => remove the condition
			newNode.Status.Conditions = append(newNode.Status.Conditions[:i], newNode.Status.Conditions[i+1:]...)
			return nil
		}
	}

	return nil
}
