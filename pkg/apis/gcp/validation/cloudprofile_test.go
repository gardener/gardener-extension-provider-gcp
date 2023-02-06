// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package validation_test

import (
	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"

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
								Architecture: pointer.String("amd64"),
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
						"Field": Equal("machineImages"),
					})),
				))
			})

			It("should forbid unsupported machine image version configuration", func() {
				cloudProfileConfig.MachineImages[0].Versions[0].Image = ""
				cloudProfileConfig.MachineImages[0].Versions[0].Architecture = pointer.String("foo")
				machineImages[0].Versions = append(machineImages[0].Versions, core.MachineImageVersion{ExpirableVersion: core.ExpirableVersion{Version: "2.0.0"}})
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, nilPath)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("machineImages[0].versions"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("machineImages[0].versions[0].image"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal("machineImages[0].versions[0].architecture"),
					})),
				))
			})
		})
	})
})
