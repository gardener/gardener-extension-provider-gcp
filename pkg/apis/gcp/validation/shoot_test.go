// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/validation"
)

var _ = Describe("Shoot validation", func() {
	Describe("#ValidateNetworking", func() {
		var networkingPath = field.NewPath("spec", "networking")

		It("should return no error because nodes CIDR was provided", func() {
			networking := &core.Networking{
				Nodes: ptr.To("1.2.3.4/5"),
			}

			errorList := ValidateNetworking(networking, networkingPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should return an error because no nodes CIDR was provided", func() {
			networking := &core.Networking{}

			errorList := ValidateNetworking(networking, networkingPath)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.networking.nodes"),
				})),
			))
		})
	})
	Describe("#ValidateWorkers", func() {
		It("should pass successfully", func() {
			workers := []core.Worker{
				{
					Name: "foo",
					Volume: &core.Volume{
						Type:       ptr.To("some-type"),
						VolumeSize: "40Gi",
					},
					Zones: []string{"zone1"},
				},
				{
					Name: "bar",
					Volume: &core.Volume{
						Type:       ptr.To("some-type"),
						VolumeSize: "40Gi",
					},
					Zones: []string{"zone1"},
				},
			}
			workers[0].Kubernetes = &core.WorkerKubernetes{Version: ptr.To("1.28.0")}

			errorList := ValidateWorkers(workers, field.NewPath(""))

			Expect(errorList).To(BeEmpty())
		})
	})
})
