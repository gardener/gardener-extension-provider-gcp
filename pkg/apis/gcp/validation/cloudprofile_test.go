// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
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
	Describe("Mixed format validation", func() {
		var (
			capabilitiesDefinitions []gardencorev1beta1.CapabilityDefinition
			cloudProfileConfig      *apisgcp.CloudProfileConfig
			machineImages           []core.MachineImage
			nilPath                 *field.Path
		)

		BeforeEach(func() {
			capabilitiesDefinitions = []gardencorev1beta1.CapabilityDefinition{{
				Name:   v1beta1constants.ArchitectureName,
				Values: []string{v1beta1constants.ArchitectureAMD64, v1beta1constants.ArchitectureARM64},
			}}
		})

		It("should allow mixed format with some versions using old format and others using new format", func() {
			cloudProfileConfig = &apisgcp.CloudProfileConfig{
				MachineImages: []apisgcp.MachineImages{
					{
						Name: "ubuntu",
						Versions: []apisgcp.MachineImageVersion{
							// Version 1.0.0 uses new format (capabilityFlavors)
							{
								Version: "1.0.0",
								CapabilityFlavors: []apisgcp.MachineImageFlavor{
									{Image: "path/to/gcp/image-amd64", Capabilities: gardencorev1beta1.Capabilities{v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureAMD64}}},
								},
							},
							// Version 1.0.1 uses old format (image with architecture)
							{Version: "1.0.1", Image: "path/to/gcp/image-amd64", Architecture: ptr.To(v1beta1constants.ArchitectureAMD64)},
							{Version: "1.0.1", Image: "path/to/gcp/image-arm64", Architecture: ptr.To(v1beta1constants.ArchitectureARM64)},
						},
					},
				},
			}
			machineImages = []core.MachineImage{
				{
					Name: "ubuntu",
					Versions: []core.MachineImageVersion{
						{
							ExpirableVersion:  core.ExpirableVersion{Version: "1.0.0"},
							CapabilityFlavors: []core.MachineImageFlavor{{Capabilities: core.Capabilities{v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureAMD64}}}},
						},
						{
							ExpirableVersion:  core.ExpirableVersion{Version: "1.0.1"},
							CapabilityFlavors: []core.MachineImageFlavor{{Capabilities: core.Capabilities{v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureAMD64}}}, {Capabilities: core.Capabilities{v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureARM64}}}},
						},
					},
				},
			}
			errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinitions, nilPath)
			Expect(errorList).To(BeEmpty())
		})

		It("should reject version with both old and new format simultaneously", func() {
			cloudProfileConfig = &apisgcp.CloudProfileConfig{
				MachineImages: []apisgcp.MachineImages{
					{
						Name: "ubuntu",
						Versions: []apisgcp.MachineImageVersion{
							{
								Version:           "1.0.0",
								Image:             "path/to/gcp/image", // old format
								Architecture:      ptr.To(v1beta1constants.ArchitectureAMD64),
								CapabilityFlavors: []apisgcp.MachineImageFlavor{{Image: "path/to/gcp/image"}}, // new format
							},
						},
					},
				},
			}
			machineImages = []core.MachineImage{
				{
					Name: "ubuntu",
					Versions: []core.MachineImageVersion{
						{
							ExpirableVersion: core.ExpirableVersion{Version: "1.0.0"},
							Architectures:    []string{v1beta1constants.ArchitectureAMD64},
						},
					},
				},
			}
			errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinitions, nilPath)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("providerConfig.machineImages[0].versions[0].image"),
				})),
			))
		})

		It("should allow old format (image with architecture) when CloudProfile uses capabilities", func() {
			cloudProfileConfig = &apisgcp.CloudProfileConfig{
				MachineImages: []apisgcp.MachineImages{
					{
						Name: "ubuntu",
						Versions: []apisgcp.MachineImageVersion{
							{Version: "1.0.0", Image: "path/to/gcp/image-amd64", Architecture: ptr.To(v1beta1constants.ArchitectureAMD64)},
						},
					},
				},
			}
			machineImages = []core.MachineImage{
				{
					Name: "ubuntu",
					Versions: []core.MachineImageVersion{
						{
							ExpirableVersion:  core.ExpirableVersion{Version: "1.0.0"},
							CapabilityFlavors: []core.MachineImageFlavor{{Capabilities: core.Capabilities{v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureAMD64}}}},
						},
					},
				},
			}
			errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinitions, nilPath)
			Expect(errorList).To(BeEmpty())
		})

		It("should report missing architecture mapping when using old format with capabilities", func() {
			cloudProfileConfig = &apisgcp.CloudProfileConfig{
				MachineImages: []apisgcp.MachineImages{
					{
						Name: "ubuntu",
						Versions: []apisgcp.MachineImageVersion{
							// Only amd64, but spec expects both amd64 and arm64
							{Version: "1.0.0", Image: "path/to/gcp/image-amd64", Architecture: ptr.To(v1beta1constants.ArchitectureAMD64)},
						},
					},
				},
			}
			machineImages = []core.MachineImage{
				{
					Name: "ubuntu",
					Versions: []core.MachineImageVersion{
						{
							ExpirableVersion: core.ExpirableVersion{Version: "1.0.0"},
							CapabilityFlavors: []core.MachineImageFlavor{
								{Capabilities: core.Capabilities{v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureAMD64}}},
								{Capabilities: core.Capabilities{v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureARM64}}},
							},
						},
					},
				},
			}
			errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinitions, nilPath)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Detail": ContainSubstring("[arm64]] is not defined in the providerConfig"),
				})),
			))
		})
	})

	DescribeTableSubtree("#ValidateCloudProfileConfig", func(isCapabilitiesCloudProfile bool) {
		var (
			capabilitiesDefinitions []gardencorev1beta1.CapabilityDefinition
			cloudProfileConfig      *apisgcp.CloudProfileConfig
			machineImages           []core.MachineImage
			nilPath                 *field.Path
			machineImageName        string
			machineImageVersion     string
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
				capabilitiesDefinitions = []gardencorev1beta1.CapabilityDefinition{{
					Name:   v1beta1constants.ArchitectureName,
					Values: []string{v1beta1constants.ArchitectureAMD64},
				}}
				providerImageVersion = apisgcp.MachineImageVersion{
					Version:           machineImageVersion,
					CapabilityFlavors: []apisgcp.MachineImageFlavor{{Image: "path/to/gcp/image"}},
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
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinitions, nilPath)

				Expect(errorList).To(BeEmpty())
			})

			It("should not require a machine image mapping because no versions are configured", func() {
				machineImages = append(machineImages, core.MachineImage{
					Name:     "suse",
					Versions: nil,
				})
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinitions, nilPath)

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
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinitions, nilPath)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("machineImages[1]"),
					})),
				))
			})

			It("should forbid missing architecture or capabilityFlavor mapping", func() {
				var versionMatcher types.GomegaMatcher

				if isCapabilitiesCloudProfile {
					versionMatcher = Equal("machineImages[0].versions[0].capabilityFlavors[0]")
					machineImages[0].Versions[0].CapabilityFlavors = []core.MachineImageFlavor{
						{Capabilities: core.Capabilities{v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureARM64}}},
					}
				} else {
					versionMatcher = Equal("machineImages[0].versions[0]")
					machineImages[0].Versions[0].Architectures = []string{"arm64"}
				}
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinitions, nilPath)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  versionMatcher,
						"Detail": ContainSubstring("is not defined in the providerConfig"),
					})),
				))
			})

			It("should forbid unsupported machine image version configuration", func() {
				var imageMatcher, capabilitiesMatcher, versionMatcher types.GomegaMatcher

				if isCapabilitiesCloudProfile {
					imageMatcher = Equal("providerConfig.machineImages[0].versions[0].capabilityFlavors[0].image")
					versionMatcher = Equal("machineImages[0].versions[0].capabilityFlavors[0]")
					capabilitiesMatcher = Equal("providerConfig.machineImages[0].versions[0].capabilityFlavors[0].capabilities.architecture[0]")
					cloudProfileConfig.MachineImages[0].Versions[0].CapabilityFlavors[0].Image = ""
					cloudProfileConfig.MachineImages[0].Versions[0].CapabilityFlavors[0].Capabilities = gardencorev1beta1.Capabilities{v1beta1constants.ArchitectureName: []string{"foo"}}
				} else {
					imageMatcher = Equal("providerConfig.machineImages[0].versions[0].image")
					versionMatcher = Equal("machineImages[0].versions[0]")
					capabilitiesMatcher = Equal("providerConfig.machineImages[0].versions[0].architecture")
					cloudProfileConfig.MachineImages[0].Versions[0].Image = ""
					cloudProfileConfig.MachineImages[0].Versions[0].Architecture = ptr.To("foo")

				}
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinitions, nilPath)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  versionMatcher,
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
		Entry("CloudProfile uses capabilityFlavors", true),
	)
})
