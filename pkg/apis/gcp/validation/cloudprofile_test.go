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
	"github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	. "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/validation"
)

var _ = Describe("CloudProfileConfig validation", func() {
	DescribeTableSubtree("#ValidateCloudProfileConfig", func(isCapabilitiesCloudProfile bool) {
		var (
			capabilitiesDefinition core.Capabilities
			cloudProfileConfig     *apisgcp.CloudProfileConfig
			machineImages          []core.MachineImage
			nilPath                *field.Path
			machineImageName       string
			machineImageVersion    string
		)

		BeforeEach(func() {
			machineImageName = "ubuntu"
			machineImageVersion = "1.2.3"
			providerImageVersion := apisgcp.MachineImageVersion{
				Version:      machineImageVersion,
				Image:        "path/to/gcp/image",
				Architecture: ptr.To(v1beta1constants.ArchitectureAMD64),
			}
			if isCapabilitiesCloudProfile {
				capabilitiesDefinition = core.Capabilities{
					v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureAMD64},
				}
				providerImageVersion = apisgcp.MachineImageVersion{
					Version:        machineImageVersion,
					CapabilitySets: []apisgcp.CapabilitySet{{Image: "path/to/gcp/image"}},
				}
			}

			cloudProfileConfig = &apisgcp.CloudProfileConfig{
				MachineImages: []apisgcp.MachineImages{
					{
						Name:     machineImageName,
						Versions: []apisgcp.MachineImageVersion{providerImageVersion},
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
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinition, nilPath)

				Expect(errorList).To(BeEmpty())
			})

			It("should not require a machine image mapping because no versions are configured", func() {
				machineImages = append(machineImages, core.MachineImage{
					Name:     "suse",
					Versions: nil,
				})
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinition, nilPath)

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
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinition, nilPath)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("machineImages[1]"),
					})),
				))
			})

			It("should forbid missing architecture or capabilitySet mapping", func() {
				if isCapabilitiesCloudProfile {
					machineImages[0].Versions[0].CapabilitySets = []core.CapabilitySet{
						{Capabilities: core.Capabilities{v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureARM64}}},
					}
				} else {
					machineImages[0].Versions[0].Architectures = []string{"arm64"}
				}
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinition, nilPath)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("machineImages[0].versions[0]"),
						"Detail": ContainSubstring("is not defined in the providerConfig"),
					})),
				))
			})

			It("should forbid unsupported machine image version configuration", func() {
				var imageMatcher, capabilitiesMatcher types.GomegaMatcher

				if isCapabilitiesCloudProfile {
					imageMatcher = Equal("providerConfig.machineImages[0].versions[0].capabilitySets[0].image")
					capabilitiesMatcher = Equal("providerConfig.machineImages[0].versions[0].capabilitySets[0].capabilities.architecture[0]")
					cloudProfileConfig.MachineImages[0].Versions[0].CapabilitySets[0].Image = ""
					cloudProfileConfig.MachineImages[0].Versions[0].CapabilitySets[0].Capabilities = core.Capabilities{v1beta1constants.ArchitectureName: []string{"foo"}}
				} else {
					imageMatcher = Equal("providerConfig.machineImages[0].versions[0].image")
					capabilitiesMatcher = Equal("providerConfig.machineImages[0].versions[0].architecture")
					cloudProfileConfig.MachineImages[0].Versions[0].Image = ""
					cloudProfileConfig.MachineImages[0].Versions[0].Architecture = ptr.To("foo")

				}
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinition, nilPath)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("machineImages[0].versions[0]"),
						"Detail": ContainSubstring("is not defined in the providerConfig"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  imageMatcher,
						"Detail": ContainSubstring("must provide the image field for image version ubuntu@1.2.3"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeNotSupported),
						"Field":  capabilitiesMatcher,
						"Detail": ContainSubstring("supported values: \"amd64\""),
					})),
				))
			})
		})
	},
		Entry("CloudProfile uses legacy versions", false),
		Entry("CloudProfile uses capabilitySets", true),
	)
})
