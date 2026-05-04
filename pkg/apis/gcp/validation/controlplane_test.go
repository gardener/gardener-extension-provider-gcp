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
			func(name string, expectErr bool) {
				controlPlane.Storage = &apisgcp.Storage{DefaultStorageClass: &name}
				errorList := ValidateControlPlaneConfig(controlPlane, allowedZones, workerZones, "", fldPath)
				if expectErr {
					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal("storage.defaultStorageClass"),
					}))))
				} else {
					Expect(errorList).To(BeEmpty())
				}
			},
			Entry("valid: default", "default", false),
			Entry("valid: gce-sc-hdd", "gce-sc-hdd", false),
			Entry("valid: gce-sc-fast", "gce-sc-fast", false),
			Entry("valid: gce-sc-hd-balanced", "gce-sc-hd-balanced", false),
			Entry("valid: gce-sc-hd-throughput", "gce-sc-hd-throughput", false),
			Entry("valid: gce-sc-hd-extreme", "gce-sc-hd-extreme", false),
			Entry("invalid: unknown", "pd-ssd", true),
		)

		DescribeTable("should validate HyperDiskBalanced",
			func(iops *int64, throughput *string, expectErrFields []string) {
				controlPlane.Storage = &apisgcp.Storage{
					HyperDiskBalanced: &apisgcp.HyperDiskConfig{
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
			Entry("valid: iops only", ptr.To[int64](3000), nil, nil),
			Entry("valid: throughput only", nil, ptr.To("140Mi"), nil),
			Entry("valid: both iops and throughput", ptr.To[int64](3000), ptr.To("140Mi"), nil),
			Entry("invalid: iops <= 0", ptr.To[int64](0), nil, []string{"storage.hyperDiskBalanced.provisionedIopsOnCreate"}),
			Entry("invalid: bad throughput quantity", nil, ptr.To("notaquantity!"), []string{"storage.hyperDiskBalanced.provisionedThroughputOnCreate"}),
		)

		DescribeTable("should validate HyperDiskThroughput",
			func(iops *int64, throughput *string, expectErrFields []string) {
				controlPlane.Storage = &apisgcp.Storage{
					HyperDiskThroughput: &apisgcp.HyperDiskConfig{
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
			Entry("valid: throughput only", nil, ptr.To("140Mi"), nil),
			Entry("invalid: iops not supported", ptr.To[int64](3000), nil, []string{"storage.hyperDiskThroughput.provisionedIopsOnCreate"}),
			Entry("invalid: bad throughput quantity", nil, ptr.To("notaquantity!"), []string{"storage.hyperDiskThroughput.provisionedThroughputOnCreate"}),
		)

		DescribeTable("should validate HyperDiskExtreme",
			func(iops *int64, throughput *string, expectErrFields []string) {
				controlPlane.Storage = &apisgcp.Storage{
					HyperDiskExtreme: &apisgcp.HyperDiskConfig{
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
			Entry("valid: iops only", ptr.To[int64](10000), nil, nil),
			Entry("invalid: throughput not supported", nil, ptr.To("140Mi"), []string{"storage.hyperDiskExtreme.provisionedThroughputOnCreate"}),
			Entry("invalid: iops <= 0", ptr.To[int64](-1), nil, []string{"storage.hyperDiskExtreme.provisionedIopsOnCreate"}),
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
