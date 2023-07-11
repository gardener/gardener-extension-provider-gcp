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

package shoot

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Mutator", func() {
	DescribeTable("#mutateNginxIngressControllerConfigMap",
		func(new *corev1.Node, old *corev1.Node) {
			mutator := &mutator{}
			err := mutator.mutateNetworkUnavailableNodeCondition(context.TODO(), new, old, func() {})

			Expect(err).To(Not(HaveOccurred()))
			if new != nil {
				for _, c := range new.Status.Conditions {
					if c.Type == corev1.NodeNetworkUnavailable {
						Expect(c.Status).To(Equal(corev1.ConditionFalse))
					}
				}
			}
		},

		Entry("no data", nil, nil),
		Entry("partial data, only new", &corev1.Node{Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionFalse}}}}, nil),
		Entry("partial data, only old", nil, &corev1.Node{Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionFalse}}}}),
		Entry("full data with condition set to false",
			&corev1.Node{Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionFalse}}}},
			&corev1.Node{Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionFalse}}}}),
		Entry("full data, updating condition set to true",
			&corev1.Node{Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionTrue, Reason: noRouteCreated, Message: nodeCreatedWithoutRoute}}}},
			&corev1.Node{Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionFalse}}}}),
		Entry("full data, updating condition set to true without previous value",
			&corev1.Node{Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionTrue, Reason: noRouteCreated, Message: nodeCreatedWithoutRoute}}}},
			&corev1.Node{Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{}}}),
	)
})
