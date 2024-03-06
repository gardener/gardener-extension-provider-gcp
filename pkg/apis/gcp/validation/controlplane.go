// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	featurevalidation "github.com/gardener/gardener/pkg/utils/validation/features"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
)

// ValidateControlPlaneConfig validates a ControlPlaneConfig object.
func ValidateControlPlaneConfig(controlPlaneConfig *apisgcp.ControlPlaneConfig, allowedZones, workerZones sets.Set[string], version string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(controlPlaneConfig.Zone) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("zone"), "must provide the name of a zone in this region"))
	} else if ok, validZones := validateZoneConstraints(allowedZones, controlPlaneConfig.Zone); !ok {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("zone"), controlPlaneConfig.Zone, validZones))
	}

	if !workerZones.Has(controlPlaneConfig.Zone) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("zone"), controlPlaneConfig.Zone, "must be part of at least one worker zone"))
	}

	if controlPlaneConfig.CloudControllerManager != nil {
		allErrs = append(allErrs, featurevalidation.ValidateFeatureGates(controlPlaneConfig.CloudControllerManager.FeatureGates, version, fldPath.Child("cloudControllerManager", "featureGates"))...)
	}

	return allErrs
}

// ValidateControlPlaneConfigUpdate validates a ControlPlaneConfig object.
func ValidateControlPlaneConfigUpdate(oldConfig, newConfig *apisgcp.ControlPlaneConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newConfig.Zone, oldConfig.Zone, fldPath.Child("zone"))...)

	return allErrs
}

func validateZoneConstraints(allowedZones sets.Set[string], zone string) (bool, []string) {
	if allowedZones.Has(zone) {
		return true, nil
	}

	return false, allowedZones.UnsortedList()
}
