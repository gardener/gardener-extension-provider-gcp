// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
