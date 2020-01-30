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

package validation

import (
	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateControlPlaneConfig validates a ControlPlaneConfig object.
func ValidateControlPlaneConfig(controlPlaneConfig *apisgcp.ControlPlaneConfig, region string, regions []gardencorev1beta1.Region) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(controlPlaneConfig.Zone) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("zone"), "must provide the name of a zone in this region"))
	} else if ok, validZones := validateZoneConstraints(regions, region, controlPlaneConfig.Zone, ""); !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("zone"), controlPlaneConfig.Zone, validZones))
	}

	return allErrs
}

// ValidateControlPlaneConfigUpdate validates a ControlPlaneConfig object.
func ValidateControlPlaneConfigUpdate(oldConfig, newConfig *apisgcp.ControlPlaneConfig, region string, regions []gardencorev1beta1.Region) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newConfig.Zone, oldConfig.Zone, field.NewPath("zone"))...)

	return allErrs
}

func validateZoneConstraints(regions []gardencorev1beta1.Region, region, zone, oldZone string) (bool, []string) {
	if zone == oldZone {
		return true, nil
	}

	validValues := []string{}

	for _, r := range regions {
		if r.Name != region {
			continue
		}

		for _, z := range r.Zones {
			validValues = append(validValues, z.Name)
			if z.Name == zone {
				return true, nil
			}
		}
	}

	return false, validValues
}
