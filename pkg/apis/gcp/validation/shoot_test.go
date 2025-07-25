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
	"k8s.io/apimachinery/pkg/runtime"
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

			errorList := ValidateWorkers(workers, field.NewPath("workers"))

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("workers[0].zones"),
				})),
			))
		})
	})

	Describe("#ValidateWorkersUpdate", func() {
		var workers []core.Worker
		BeforeEach(func() {
			workers = []core.Worker{
				{
					Name: "foo",
					Volume: &core.Volume{
						Type:       ptr.To("some-type"),
						VolumeSize: "40Gi",
					},
					Zones: []string{"zone1", "zone2"},
				},
				{
					Name: "bar",
					Volume: &core.Volume{
						Type:       ptr.To("some-type"),
						VolumeSize: "40Gi",
					},
					Zones: []string{"zone1", "zone2"},
				},
			}
		})

		It("should pass because workers are unchanged", func() {
			newWorkers := copyWorkers(workers)
			errorList := ValidateWorkersUpdate(workers, newWorkers, field.NewPath("workers"))
			Expect(errorList).To(BeEmpty())
		})

		It("should allow adding workers", func() {
			newWorkers := append(workers[:0:0], workers...)
			workers = workers[:1]
			errorList := ValidateWorkersUpdate(workers, newWorkers, field.NewPath("workers"))
			Expect(errorList).To(BeEmpty())
		})

		It("should allow adding a zone to a worker", func() {
			newWorkers := copyWorkers(workers)
			newWorkers[0].Zones = append(newWorkers[0].Zones, "another-zone")
			errorList := ValidateWorkersUpdate(workers, newWorkers, field.NewPath("workers"))
			Expect(errorList).To(BeEmpty())
		})

		It("should forbid removing a zone from a worker", func() {
			newWorkers := copyWorkers(workers)
			newWorkers[1].Zones = newWorkers[1].Zones[1:]
			errorList := ValidateWorkersUpdate(workers, newWorkers, field.NewPath("workers"))
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("workers[1].zones"),
				})),
			))
		})

		It("should forbid changing the zone order", func() {
			newWorkers := copyWorkers(workers)
			newWorkers[0].Zones[0] = workers[0].Zones[1]
			newWorkers[0].Zones[1] = workers[0].Zones[0]
			newWorkers[1].Zones[0] = workers[1].Zones[1]
			newWorkers[1].Zones[1] = workers[1].Zones[0]
			errorList := ValidateWorkersUpdate(workers, newWorkers, field.NewPath("workers"))
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("workers[0].zones"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("workers[1].zones"),
				})),
			))
		})

		It("should forbid adding a zone while changing an existing one", func() {
			newWorkers := copyWorkers(workers)
			newWorkers = append(newWorkers, core.Worker{Name: "worker3", Zones: []string{"zone1"}})
			newWorkers[1].Zones[0] = workers[1].Zones[1]
			errorList := ValidateWorkersUpdate(workers, newWorkers, field.NewPath("workers"))
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("workers[1].zones"),
				})),
			))
		})

		It("should forbid changing the providerConfig if the update strategy is in-place", func() {
			workers[0].UpdateStrategy = ptr.To(core.AutoInPlaceUpdate)
			workers[0].ProviderConfig = &runtime.RawExtension{
				Raw: []byte(`{"foo":"bar"}`),
			}

			workers[1].Name = "worker2"
			workers[1].UpdateStrategy = ptr.To(core.ManualInPlaceUpdate)
			workers[1].ProviderConfig = &runtime.RawExtension{
				Raw: []byte(`{"zoo":"dash"}`),
			}

			// provider config changed but update strategy is not in-place
			workers = append(workers, core.Worker{
				Name:           "worker3",
				UpdateStrategy: ptr.To(core.AutoRollingUpdate),
				ProviderConfig: &runtime.RawExtension{
					Raw: []byte(`{"bar":"foo"}`),
				},
			})

			// no change in provider config
			workers = append(workers, core.Worker{
				Name:           "worker4",
				UpdateStrategy: ptr.To(core.AutoInPlaceUpdate),
				ProviderConfig: &runtime.RawExtension{
					Raw: []byte(`{"bar":"foo"}`),
				},
			})

			newWorkers := copyWorkers(workers)
			newWorkers[0].ProviderConfig = &runtime.RawExtension{
				Raw: []byte(`{"foo":"baz"}`),
			}
			newWorkers[1].ProviderConfig = &runtime.RawExtension{
				Raw: []byte(`{"zoo":"bash"}`),
			}
			newWorkers[2].ProviderConfig = &runtime.RawExtension{
				Raw: []byte(`{"bar":"baz"}`),
			}

			Expect(ValidateWorkersUpdate(workers, newWorkers, field.NewPath("spec", "provider", "workers"))).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.provider.workers[0].providerConfig"),
					"Detail": Equal("providerConfig is immutable when update strategy is in-place"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.provider.workers[1].providerConfig"),
					"Detail": Equal("providerConfig is immutable when update strategy is in-place"),
				})),
			))
		})

		It("should forbid changing the data volumes if the update strategy is in-place", func() {
			workers[0].UpdateStrategy = ptr.To(core.AutoInPlaceUpdate)
			workers[0].DataVolumes = []core.DataVolume{
				{
					Name:       "foo",
					VolumeSize: "20Gi",
					Type:       ptr.To("foo"),
				},
			}

			workers[1].Name = "worker2"
			workers[1].UpdateStrategy = ptr.To(core.ManualInPlaceUpdate)
			workers[1].DataVolumes = []core.DataVolume{
				{
					Name:       "bar",
					VolumeSize: "30Gi",
					Type:       ptr.To("bar"),
				},
			}

			newWorkers := copyWorkers(workers)
			newWorkers[0].DataVolumes = []core.DataVolume{
				{
					Name:       "baz",
					VolumeSize: "40Gi",
					Type:       ptr.To("baz"),
				},
			}
			newWorkers[1].DataVolumes = []core.DataVolume{
				{
					Name:       "qux",
					VolumeSize: "50Gi",
					Type:       ptr.To("qux"),
				},
			}

			Expect(ValidateWorkersUpdate(workers, newWorkers, field.NewPath("spec", "provider", "workers"))).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.provider.workers[0].dataVolumes"),
					"Detail": Equal("dataVolumes are immutable when update strategy is in-place"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.provider.workers[1].dataVolumes"),
					"Detail": Equal("dataVolumes are immutable when update strategy is in-place"),
				})),
			))
		})
	})
})
