// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	featurevalidation "github.com/gardener/gardener/pkg/utils/validation/features"
	"k8s.io/apimachinery/pkg/api/resource"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
)

var validStorageClassNames = sets.New(
	"default",
	"gce-sc-hdd",
	"gce-sc-fast",
	"gce-sc-hd-balanced",
	"gce-sc-hd-throughput",
	"gce-sc-hd-extreme",
)

// hyperDiskStorageClassNames maps each hyperdisk storage class name to the function that
// retrieves the corresponding HyperDiskConfig from a Storage object.
var hyperDiskStorageClassNames = map[string]func(*apisgcp.Storage) *apisgcp.HyperDiskConfig{
	"gce-sc-hd-balanced":   func(s *apisgcp.Storage) *apisgcp.HyperDiskConfig { return s.HyperDiskBalanced },
	"gce-sc-hd-throughput": func(s *apisgcp.Storage) *apisgcp.HyperDiskConfig { return s.HyperDiskThroughput },
	"gce-sc-hd-extreme":    func(s *apisgcp.Storage) *apisgcp.HyperDiskConfig { return s.HyperDiskExtreme },
}

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

	if controlPlaneConfig.Storage != nil {
		storagePath := fldPath.Child("storage")
		if controlPlaneConfig.Storage.DefaultStorageClass != nil {
			if !validStorageClassNames.Has(*controlPlaneConfig.Storage.DefaultStorageClass) {
				allErrs = append(allErrs, field.NotSupported(storagePath.Child("defaultStorageClass"), *controlPlaneConfig.Storage.DefaultStorageClass, sets.List(validStorageClassNames)))
			} else if getConfig, ok := hyperDiskStorageClassNames[*controlPlaneConfig.Storage.DefaultStorageClass]; ok {
				cfg := getConfig(controlPlaneConfig.Storage)
				if cfg == nil || !cfg.Enabled {
					allErrs = append(allErrs, field.Invalid(storagePath.Child("defaultStorageClass"), *controlPlaneConfig.Storage.DefaultStorageClass, "the corresponding StorageClass must have enabled set to true"))
				}
			}
		}
		if controlPlaneConfig.Storage.HyperDiskBalanced != nil {
			allErrs = append(allErrs, validateHyperDiskConfig(controlPlaneConfig.Storage.HyperDiskBalanced, true, true, storagePath.Child("hyperDiskBalanced"))...)
		}
		if controlPlaneConfig.Storage.HyperDiskThroughput != nil {
			allErrs = append(allErrs, validateHyperDiskConfig(controlPlaneConfig.Storage.HyperDiskThroughput, false, true, storagePath.Child("hyperDiskThroughput"))...)
		}
		if controlPlaneConfig.Storage.HyperDiskExtreme != nil {
			allErrs = append(allErrs, validateHyperDiskConfig(controlPlaneConfig.Storage.HyperDiskExtreme, true, false, storagePath.Child("hyperDiskExtreme"))...)
		}
	}

	return allErrs
}

// validateHyperDiskConfig validates a HyperDiskConfig, enforcing which parameters are supported and required.
func validateHyperDiskConfig(cfg *apisgcp.HyperDiskConfig, iopsAllowed, throughputAllowed bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if !cfg.Enabled {
		return allErrs
	}
	if iopsAllowed && cfg.ProvisionedIopsOnCreate == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("provisionedIopsOnCreate"), "provisionedIopsOnCreate is required when enabled"))
	}
	if throughputAllowed && cfg.ProvisionedThroughputOnCreate == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("provisionedThroughputOnCreate"), "provisionedThroughputOnCreate is required when enabled"))
	}
	if cfg.ProvisionedIopsOnCreate != nil {
		if !iopsAllowed {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("provisionedIopsOnCreate"), "provisionedIopsOnCreate is not supported for this hyperdisk type"))
		} else if *cfg.ProvisionedIopsOnCreate <= 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("provisionedIopsOnCreate"), *cfg.ProvisionedIopsOnCreate, "must be a positive integer"))
		}
	}
	if cfg.ProvisionedThroughputOnCreate != nil {
		if !throughputAllowed {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("provisionedThroughputOnCreate"), "provisionedThroughputOnCreate is not supported for this hyperdisk type"))
		} else if _, err := resource.ParseQuantity(*cfg.ProvisionedThroughputOnCreate); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("provisionedThroughputOnCreate"), *cfg.ProvisionedThroughputOnCreate, "must be a valid quantity string (e.g. \"140Mi\")"))
		}
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
