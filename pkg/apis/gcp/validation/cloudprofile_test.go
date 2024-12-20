// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	. "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/validation"
)

var _ = Describe("CloudProfileConfig validation", func() {
	Describe("#ValidateCloudProfileConfig", func() {
		var (
			cloudProfileConfig  *apisgcp.CloudProfileConfig
			machineImages       []core.MachineImage
			nilPath             *field.Path
			machineImageName    string
			machineImageVersion string
		)

		BeforeEach(func() {
			machineImageName = "ubuntu"
			machineImageVersion = "1.2.3"
			cloudProfileConfig = &apisgcp.CloudProfileConfig{
				MachineImages: []apisgcp.MachineImages{
					{
						Name: machineImageName,
						Versions: []apisgcp.MachineImageVersion{
							{
								Version:      machineImageVersion,
								Image:        "path/to/gcp/image",
								Architecture: ptr.To(v1beta1constants.ArchitectureAMD64),
							},
						},
					},
				},
			}
			machineImages = []core.MachineImage{
				{
					Name: machineImageName,
					Versions: []core.MachineImageVersion{
						{
							ExpirableVersion: core.ExpirableVersion{Version: machineImageVersion},
							Architectures:    []string{v1beta1constants.ArchitectureAMD64},
						},
					},
				},
			}
		})

		Context("machine image validation", func() {
			It("should pass validation", func() {
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, nilPath)

				Expect(errorList).To(BeEmpty())
			})

			It("should not require a machine image mapping because no versions are configured", func() {
				machineImages = append(machineImages, core.MachineImage{
					Name:     "suse",
					Versions: nil,
				})
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, nilPath)

				Expect(errorList).To(BeEmpty())
			})

			It("should require a machine image mapping to be configured", func() {
				machineImages = append(machineImages, core.MachineImage{
					Name: "suse",
					Versions: []core.MachineImageVersion{
						{
							ExpirableVersion: core.ExpirableVersion{
								Version: machineImageVersion,
							},
						},
					},
				})
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, nilPath)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("machineImages[1]"),
					})),
				))
			})

			It("should forbid missing architecture mapping", func() {
				machineImages[0].Versions[0].Architectures = []string{"arm64"}
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, nilPath)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("machineImages[0].versions[0]"),
					})),
				))
			})

			It("should forbid unsupported machine image version configuration", func() {
				cloudProfileConfig.MachineImages[0].Versions[0].Image = ""
				cloudProfileConfig.MachineImages[0].Versions[0].Architecture = ptr.To("foo")
				machineImages[0].Versions = append(machineImages[0].Versions, core.MachineImageVersion{ExpirableVersion: core.ExpirableVersion{Version: "2.0.0"}})
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, nilPath)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("machineImages[0].versions[0]"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("providerConfig.machineImages[0].versions[0].image"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal("providerConfig.machineImages[0].versions[0].architecture"),
					})),
				))
			})
		})
	})
})
