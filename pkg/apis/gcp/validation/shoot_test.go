// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"github.com/Masterminds/semver/v3"
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
		networkingPath := field.NewPath("spec", "networking")

		It("should return no error because nodes CIDR was provided", func() {
			networking := &core.Networking{
				Nodes: ptr.To("1.2.3.4/5"),
			}

			errorList := ValidateNetworking(networking, networkingPath, nil)

			Expect(errorList).To(BeEmpty())
		})

		It("should return an error because no nodes CIDR was provided", func() {
			networking := &core.Networking{}

			errorList := ValidateNetworking(networking, networkingPath, nil)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.networking.nodes"),
				})),
			))
		})

		Describe("#ValidateNetworkingUpdate", func() {
			var networkingPath = field.NewPath("spec", "networking")

			It("should return no error because ipFamilies update was valid", func() {
				oldNetworking := &core.Networking{
					IPFamilies: []core.IPFamily{core.IPFamilyIPv4},
				}
				newNetworking := &core.Networking{
					IPFamilies: []core.IPFamily{core.IPFamilyIPv4, core.IPFamilyIPv6},
				}

				errorList := ValidateNetworkingConfigUpdate(oldNetworking, newNetworking, networkingPath)

				Expect(errorList).To(BeEmpty())
			})

			It("should return error because ipFamilies update was not valid", func() {
				oldNetworking := &core.Networking{
					IPFamilies: []core.IPFamily{core.IPFamilyIPv4},
				}
				newNetworking := &core.Networking{
					IPFamilies: []core.IPFamily{core.IPFamilyIPv6},
				}

				errorList := ValidateNetworkingConfigUpdate(oldNetworking, newNetworking, networkingPath)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.networking.ipFamilies"),
					})),
				))
			})

		})

		Describe("dual-stack", func() {
			var (
				networking              *core.Networking
				validDualStackVersion   *semver.Version
				invalidDualStackVersion *semver.Version
			)

			BeforeEach(func() {
				networking = &core.Networking{
					Nodes: ptr.To("1.2.3.4/5"),
					IPFamilies: []core.IPFamily{
						core.IPFamilyIPv4,
						core.IPFamilyIPv6,
					},
				}

				var err error
				validDualStackVersion, err = semver.NewVersion("1.31.7")
				Expect(err).To(Succeed())
				invalidDualStackVersion, err = semver.NewVersion("1.30.12")
				Expect(err).To(Succeed())
			})

			It("should return an error for dual-stack because kubernetes release is too old", func() {
				errorList := ValidateNetworking(networking, networkingPath, invalidDualStackVersion)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.networking.ipFamilies"),
					})),
				))
			})

			It("should pass dual-stack", func() {
				errorList := ValidateNetworking(networking, networkingPath, validDualStackVersion)

				Expect(errorList).To(BeEmpty())
			})
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

		It("should fail without at least one zone", func() {
			workers := []core.Worker{
				{
					Name: "bar",
					Volume: &core.Volume{
						Type:       ptr.To("some-type"),
						VolumeSize: "40Gi",
					},
					Zones: []string{},
				},
			}
			workers[0].Kubernetes = &core.WorkerKubernetes{Version: ptr.To("1.28.0")}

			errorList := ValidateWorkers(workers, field.NewPath("workers"))

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("workers[0].zones"),
				})),
			))
		})
	})
})
