// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	. "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/validation"
)

var _ = Describe("ControlPlaneConfig validation", func() {
	var (
		zone         = "some-zone"
		allowedZones = sets.New("zone1", "zone2", "some-zone")
		workerZones  = sets.New("zone1", "zone2", "some-zone")
		controlPlane *apisgcp.ControlPlaneConfig
		fldPath      *field.Path
	)

	BeforeEach(func() {
		controlPlane = &apisgcp.ControlPlaneConfig{
			Zone: zone,
		}
	})

	Describe("#ValidateControlPlaneConfig", func() {
		It("should return no errors for a valid configuration", func() {
			Expect(ValidateControlPlaneConfig(controlPlane, allowedZones, workerZones, "", fldPath)).To(BeEmpty())
		})

		It("should require that the control-plane config zone be part of the worker pool zone configuration", func() {
			controlPlane.Zone = ""
			workerZonesNotSupported := sets.New("zone3", "zone4")
			errorList := ValidateControlPlaneConfig(controlPlane, allowedZones, workerZonesNotSupported, "", fldPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("zone"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("zone"),
			}))))
		})

		It("should require the name of a zone", func() {
			controlPlane.Zone = ""

			errorList := ValidateControlPlaneConfig(controlPlane, allowedZones, workerZones, "", fldPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("zone"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("zone"),
			}))))
		})

		It("should require a name of a zone that is part of the regions", func() {
			controlPlane.Zone = "bar"

			errorList := ValidateControlPlaneConfig(controlPlane, allowedZones, workerZones, "", fldPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("zone"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("zone"),
			}))))
		})

		It("should fail with invalid CCM feature gates", func() {
			controlPlane.CloudControllerManager = &apisgcp.CloudControllerManagerConfig{
				FeatureGates: map[string]bool{
					"AnyVolumeDataSource": true,
					"Foo":                 true,
				},
			}

			errorList := ValidateControlPlaneConfig(controlPlane, allowedZones, workerZones, "1.30.14", fldPath)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("cloudControllerManager.featureGates.Foo"),
				})),
			))
		})

		DescribeTable("should validate DefaultStorageClass",
			func(name string, storage *apisgcp.Storage, expectErrType field.ErrorType) {
				controlPlane.Storage = storage
				controlPlane.Storage.DefaultStorageClass = &name
				errorList := ValidateControlPlaneConfig(controlPlane, allowedZones, workerZones, "", fldPath)
				if expectErrType == "" {
					Expect(errorList).To(BeEmpty())
				} else {
					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(expectErrType),
						"Field": Equal("storage.defaultStorageClass"),
					}))))
				}
			},
			Entry("valid: default", "default", &apisgcp.Storage{}, field.ErrorType("")),
			Entry("valid: gce-sc-hdd", "gce-sc-hdd", &apisgcp.Storage{}, field.ErrorType("")),
			Entry("valid: gce-sc-fast", "gce-sc-fast", &apisgcp.Storage{}, field.ErrorType("")),
			Entry("valid: gce-sc-hd-balanced with enabled config", "gce-sc-hd-balanced", &apisgcp.Storage{
				HyperDiskBalanced: &apisgcp.HyperDiskConfig{Enabled: true, ProvisionedIopsOnCreate: ptr.To[int64](3000), ProvisionedThroughputOnCreate: ptr.To("140Mi")},
			}, field.ErrorType("")),
			Entry("valid: gce-sc-hd-throughput with enabled config", "gce-sc-hd-throughput", &apisgcp.Storage{
				HyperDiskThroughput: &apisgcp.HyperDiskConfig{Enabled: true, ProvisionedThroughputOnCreate: ptr.To("250Mi")},
			}, field.ErrorType("")),
			Entry("valid: gce-sc-hd-extreme with enabled config", "gce-sc-hd-extreme", &apisgcp.Storage{
				HyperDiskExtreme: &apisgcp.HyperDiskConfig{Enabled: true, ProvisionedIopsOnCreate: ptr.To[int64](10000)},
			}, field.ErrorType("")),
			Entry("invalid: unknown", "pd-ssd", &apisgcp.Storage{}, field.ErrorTypeNotSupported),
			Entry("invalid: gce-sc-hd-balanced without hyperdisk config", "gce-sc-hd-balanced", &apisgcp.Storage{}, field.ErrorTypeInvalid),
			Entry("invalid: gce-sc-hd-balanced with disabled config", "gce-sc-hd-balanced", &apisgcp.Storage{
				HyperDiskBalanced: &apisgcp.HyperDiskConfig{Enabled: false},
			}, field.ErrorTypeInvalid),
			Entry("invalid: gce-sc-hd-throughput without hyperdisk config", "gce-sc-hd-throughput", &apisgcp.Storage{}, field.ErrorTypeInvalid),
			Entry("invalid: gce-sc-hd-extreme without hyperdisk config", "gce-sc-hd-extreme", &apisgcp.Storage{}, field.ErrorTypeInvalid),
		)

		DescribeTable("should validate HyperDiskBalanced",
			func(enabled bool, iops *int64, throughput *string, expectErrFields []string) {
				controlPlane.Storage = &apisgcp.Storage{
					HyperDiskBalanced: &apisgcp.HyperDiskConfig{
						Enabled:                       enabled,
						ProvisionedIopsOnCreate:       iops,
						ProvisionedThroughputOnCreate: throughput,
					},
				}
				errorList := ValidateControlPlaneConfig(controlPlane, allowedZones, workerZones, "", fldPath)
				if len(expectErrFields) == 0 {
					Expect(errorList).To(BeEmpty())
				} else {
					fields := make([]interface{}, 0, len(expectErrFields))
					for _, f := range expectErrFields {
						f := f
						fields = append(fields, PointTo(MatchFields(IgnoreExtras, Fields{"Field": Equal(f)})))
					}
					Expect(errorList).To(ConsistOf(fields...))
				}
			},
			Entry("disabled: no fields required", false, nil, nil, nil),
			Entry("enabled: iops and throughput provided", true, ptr.To[int64](3000), ptr.To("140Mi"), nil),
			Entry("enabled: missing iops", true, nil, ptr.To("140Mi"), []string{"storage.hyperDiskBalanced.provisionedIopsOnCreate"}),
			Entry("enabled: missing throughput", true, ptr.To[int64](3000), nil, []string{"storage.hyperDiskBalanced.provisionedThroughputOnCreate"}),
			Entry("enabled: missing both", true, nil, nil, []string{"storage.hyperDiskBalanced.provisionedIopsOnCreate", "storage.hyperDiskBalanced.provisionedThroughputOnCreate"}),
			Entry("enabled: iops <= 0", true, ptr.To[int64](0), ptr.To("140Mi"), []string{"storage.hyperDiskBalanced.provisionedIopsOnCreate"}),
			Entry("enabled: bad throughput quantity", true, ptr.To[int64](3000), ptr.To("notaquantity!"), []string{"storage.hyperDiskBalanced.provisionedThroughputOnCreate"}),
		)

		DescribeTable("should validate HyperDiskThroughput",
			func(enabled bool, iops *int64, throughput *string, expectErrFields []string) {
				controlPlane.Storage = &apisgcp.Storage{
					HyperDiskThroughput: &apisgcp.HyperDiskConfig{
						Enabled:                       enabled,
						ProvisionedIopsOnCreate:       iops,
						ProvisionedThroughputOnCreate: throughput,
					},
				}
				errorList := ValidateControlPlaneConfig(controlPlane, allowedZones, workerZones, "", fldPath)
				if len(expectErrFields) == 0 {
					Expect(errorList).To(BeEmpty())
				} else {
					fields := make([]interface{}, 0, len(expectErrFields))
					for _, f := range expectErrFields {
						f := f
						fields = append(fields, PointTo(MatchFields(IgnoreExtras, Fields{"Field": Equal(f)})))
					}
					Expect(errorList).To(ConsistOf(fields...))
				}
			},
			Entry("disabled: no fields required", false, nil, nil, nil),
			Entry("enabled: throughput provided", true, nil, ptr.To("250Mi"), nil),
			Entry("enabled: missing throughput", true, nil, nil, []string{"storage.hyperDiskThroughput.provisionedThroughputOnCreate"}),
			Entry("enabled: iops not supported", true, ptr.To[int64](3000), ptr.To("250Mi"), []string{"storage.hyperDiskThroughput.provisionedIopsOnCreate"}),
			Entry("enabled: bad throughput quantity", true, nil, ptr.To("notaquantity!"), []string{"storage.hyperDiskThroughput.provisionedThroughputOnCreate"}),
		)

		DescribeTable("should validate HyperDiskExtreme",
			func(enabled bool, iops *int64, throughput *string, expectErrFields []string) {
				controlPlane.Storage = &apisgcp.Storage{
					HyperDiskExtreme: &apisgcp.HyperDiskConfig{
						Enabled:                       enabled,
						ProvisionedIopsOnCreate:       iops,
						ProvisionedThroughputOnCreate: throughput,
					},
				}
				errorList := ValidateControlPlaneConfig(controlPlane, allowedZones, workerZones, "", fldPath)
				if len(expectErrFields) == 0 {
					Expect(errorList).To(BeEmpty())
				} else {
					fields := make([]interface{}, 0, len(expectErrFields))
					for _, f := range expectErrFields {
						f := f
						fields = append(fields, PointTo(MatchFields(IgnoreExtras, Fields{"Field": Equal(f)})))
					}
					Expect(errorList).To(ConsistOf(fields...))
				}
			},
			Entry("disabled: no fields required", false, nil, nil, nil),
			Entry("enabled: iops provided", true, ptr.To[int64](10000), nil, nil),
			Entry("enabled: missing iops", true, nil, nil, []string{"storage.hyperDiskExtreme.provisionedIopsOnCreate"}),
			Entry("enabled: throughput not supported", true, ptr.To[int64](10000), ptr.To("140Mi"), []string{"storage.hyperDiskExtreme.provisionedThroughputOnCreate"}),
			Entry("enabled: iops <= 0", true, ptr.To[int64](-1), nil, []string{"storage.hyperDiskExtreme.provisionedIopsOnCreate"}),
		)
	})

	Describe("#ValidateControlPlaneConfigUpdate", func() {
		It("should return no errors for an unchanged config", func() {
			Expect(ValidateControlPlaneConfigUpdate(controlPlane, controlPlane, fldPath)).To(BeEmpty())
		})

		It("should forbid changing the zone", func() {
			newControlPlane := controlPlane.DeepCopy()
			newControlPlane.Zone = "foo"

			errorList := ValidateControlPlaneConfigUpdate(controlPlane, newControlPlane, fldPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("zone"),
			}))))
		})
	})
})
