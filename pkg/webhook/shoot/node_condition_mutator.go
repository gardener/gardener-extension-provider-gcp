// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
