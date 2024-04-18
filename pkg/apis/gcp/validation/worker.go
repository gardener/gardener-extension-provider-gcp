// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"strings"

	"github.com/gardener/gardener/pkg/apis/core"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
)

// VolumeTypeScratch is the gcp SCRATCH volume type
const VolumeTypeScratch = "SCRATCH"

var validVolumeLocalSSDInterfacesTypes = sets.New("NVME", "SCSI")

// ValidateWorkerConfig validates a WorkerConfig object.
func ValidateWorkerConfig(workerConfig *gcp.WorkerConfig, dataVolumes []core.DataVolume) field.ErrorList {
	allErrs := field.ErrorList{}

	for _, volume := range dataVolumes {
		if volume.Type == nil {
			allErrs = append(allErrs, field.Required(field.NewPath("volume", "type"), "must not be empty"))
		}
		if volume.Type != nil && *volume.Type == VolumeTypeScratch {
			if workerConfig == nil || workerConfig.Volume == nil || workerConfig.Volume.LocalSSDInterface == nil {
				allErrs = append(allErrs, field.Required(field.NewPath("volume", "localSSDInterface"), fmt.Sprintf("must be set when using %s volumes", VolumeTypeScratch)))
			} else {
				if !validVolumeLocalSSDInterfacesTypes.Has(*workerConfig.Volume.LocalSSDInterface) {
					allErrs = append(allErrs, field.NotSupported(field.NewPath("volume", "localSSDInterface"), *workerConfig.Volume.LocalSSDInterface, validVolumeLocalSSDInterfacesTypes.UnsortedList()))
				}
			}
			// DiskEncryption not allowed for type SCRATCH
			if workerConfig != nil && workerConfig.Volume != nil && workerConfig.Volume.Encryption != nil {
				allErrs = append(allErrs, field.Invalid(field.NewPath("volume", "Encryption"), *workerConfig.Volume.Encryption, fmt.Sprintf("must not be set in combination with %s volumes", VolumeTypeScratch)))
			}
		}
		// LocalSSDInterface only allowed for type SCRATCH
		if workerConfig != nil && workerConfig.Volume != nil && workerConfig.Volume.LocalSSDInterface != nil &&
			volume.Type != nil && *volume.Type != VolumeTypeScratch {
			allErrs = append(allErrs, field.Invalid(field.NewPath("volume", "LocalSSDInterface"), *workerConfig.Volume.LocalSSDInterface, fmt.Sprintf("is only allowed for type %s", VolumeTypeScratch)))
		}
	}

	if workerConfig != nil {
		allErrs = append(allErrs, validateGPU(workerConfig.GPU, field.NewPath("gpu"))...)
		allErrs = append(allErrs, validateServiceAccount(workerConfig.ServiceAccount, field.NewPath("serviceAccount"))...)
		if workerConfig.Volume != nil {
			allErrs = append(allErrs, validateDiskEncryption(workerConfig.Volume.Encryption, field.NewPath("volume", "encryption"))...)
		}
	}

	return allErrs
}

func validateGPU(gpu *gcp.GPU, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if gpu == nil {
		return allErrs
	}

	if gpu.AcceleratorType == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("acceleratorType"), "must be set when providing gpu"))
	}

	if gpu.Count <= 0 {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("count"), "must be > 0 when providing gpu"))
	}

	return allErrs
}

func validateServiceAccount(sa *gcp.ServiceAccount, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if sa == nil {
		return allErrs
	}

	if sa.Email == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("email"), "must be set when providing service account"))
	}

	if len(sa.Scopes) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("scopes"), "must have at least one scope"))
	} else {
		existingScopes := sets.NewString()

		for i, scope := range sa.Scopes {
			switch {
			case scope == "":
				allErrs = append(allErrs, field.Required(fldPath.Child("scopes").Index(i), "must not be empty"))
			case existingScopes.Has(scope):
				allErrs = append(allErrs, field.Duplicate(fldPath.Child("scopes").Index(i), scope))
			default:
				existingScopes.Insert(scope)
			}
		}
	}

	return allErrs
}

// validateDiskEncryption validates the provider specific disk encryption configuration for a volume
func validateDiskEncryption(encryption *gcp.DiskEncryption, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if encryption == nil {
		return allErrs
	}

	if encryption.KmsKeyName == nil || strings.TrimSpace(*encryption.KmsKeyName) == "" {
		// Currently DiskEncryption only contains CMEK fields. Hence if not nil, then kmsKeyName is a must
		// Validation logic will need to be modified when CSEK fields are possibly added to gcp.DiskEncryption in the future.
		allErrs = append(allErrs, field.Required(fldPath.Child("kmsKeyName"), "must be specified when configuring disk encryption"))
	}

	return allErrs
}
