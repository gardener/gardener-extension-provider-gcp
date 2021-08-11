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
	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	. "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/validation"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var _ = Describe("ControlPlaneConfig validation", func() {
	var (
		zone         = "some-zone"
		allowedZones = sets.NewString("zone1", "zone2", "some-zone")
		workerZones  = sets.NewString("zone1", "zone2", "some-zone")
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
			workerZonesNotSupported := sets.NewString("zone3", "zone4")
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
					"AnyVolumeDataSource":      true,
					"CustomResourceValidation": true,
					"Foo":                      true,
				},
			}

			errorList := ValidateControlPlaneConfig(controlPlane, allowedZones, workerZones, "1.18.14", fldPath)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("cloudControllerManager.featureGates.CustomResourceValidation"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("cloudControllerManager.featureGates.Foo"),
				})),
			))
		})

		Context("alphaFeatures", func() {
			It("should allow alpha features", func() {
				controlPlane.CloudControllerManager = &apisgcp.CloudControllerManagerConfig{
					FeatureGates: map[string]bool{
						"AllAlpha": true,
					},
					AlphaFeatureGates: map[string]bool{
						"xyz": true,
						"abc": true,
					},
				}

				errorList := ValidateControlPlaneConfig(controlPlane, allowedZones, workerZones, "1.18.14", fldPath)

				Expect(errorList).To(HaveLen(0))
			})

			It("should not allow alpha features", func() {
				controlPlane.CloudControllerManager = &apisgcp.CloudControllerManagerConfig{
					FeatureGates: map[string]bool{},
					AlphaFeatureGates: map[string]bool{
						"xyz": true,
						"abc": true,
					},
				}

				errorList := ValidateControlPlaneConfig(controlPlane, allowedZones, workerZones, "1.18.14", fldPath)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("cloudControllerManager.alphaFeatures"),
					})),
				))
			})
		})

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
