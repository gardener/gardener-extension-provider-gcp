// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"strings"

	"github.com/gardener/gardener/pkg/apis/core"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/worker"
)

var (
	validVolumeLocalSSDInterfacesTypes = sets.New("NVME", "SCSI")

	providerFldPath   = field.NewPath("providerConfig")
	volumeFldPath     = providerFldPath.Child("volume")
	dataVolumeFldPath = providerFldPath.Child("dataVolume")
)

// ValidateWorkerConfig validates a WorkerConfig object.
func ValidateWorkerConfig(workerConfig *gcp.WorkerConfig, dataVolumes []core.DataVolume) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, dataVolume := range dataVolumes {
		dataVolumeFldPath := field.NewPath("dataVolumes").Index(i)
		allErrs = append(allErrs, validateDataVolume(workerConfig, dataVolume, dataVolumeFldPath)...)
	}

	if workerConfig != nil {
		allErrs = append(allErrs, validateGPU(workerConfig.GPU, providerFldPath.Child("gpu"))...)
		allErrs = append(allErrs, validateServiceAccount(workerConfig.ServiceAccount, providerFldPath.Child("serviceAccount"))...)
		if workerConfig.Volume != nil {
			allErrs = append(allErrs, validateDiskEncryption(workerConfig.Volume.Encryption, volumeFldPath.Child("encryption"))...)
		}
		allErrs = append(allErrs, validateNodeTemplate(workerConfig.NodeTemplate, providerFldPath.Child("nodeTemplate"))...)
		if workerConfig.DataVolumes != nil {
			allErrs = append(allErrs, validateDataVolumeConfigs(dataVolumes, workerConfig.DataVolumes)...)
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

func validateDataVolume(workerConfig *gcp.WorkerConfig, volume core.DataVolume, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if volume.Type == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "must not be empty"))
		return allErrs
	}

	allErrs = append(allErrs, validateScratchDisk(*volume.Type, workerConfig)...)

	return allErrs
}

func validateScratchDisk(volumeType string, workerConfig *gcp.WorkerConfig) field.ErrorList {
	allErrs := field.ErrorList{}

	interfacePath := volumeFldPath.Child("interface")
	encryptionPath := volumeFldPath.Child("encryption")

	if volumeType == worker.VolumeTypeScratch {
		if workerConfig == nil || workerConfig.Volume == nil || workerConfig.Volume.LocalSSDInterface == nil {
			allErrs = append(allErrs, field.Required(interfacePath, fmt.Sprintf("must be set when using %s volumes", worker.VolumeTypeScratch)))
		} else {
			if !validVolumeLocalSSDInterfacesTypes.Has(*workerConfig.Volume.LocalSSDInterface) {
				allErrs = append(allErrs, field.NotSupported(interfacePath, *workerConfig.Volume.LocalSSDInterface, validVolumeLocalSSDInterfacesTypes.UnsortedList()))
			}
		}
		// DiskEncryption not allowed for type SCRATCH
		if workerConfig != nil && workerConfig.Volume != nil && workerConfig.Volume.Encryption != nil {
			allErrs = append(allErrs, field.Invalid(encryptionPath, *workerConfig.Volume.Encryption, fmt.Sprintf("must not be set in combination with %s volumes", worker.VolumeTypeScratch)))
		}
	} else {
		// LocalSSDInterface only allowed for type SCRATCH
		if workerConfig != nil && workerConfig.Volume != nil && workerConfig.Volume.LocalSSDInterface != nil {
			allErrs = append(allErrs, field.Invalid(encryptionPath, *workerConfig.Volume.LocalSSDInterface, fmt.Sprintf("is only allowed for type %s", worker.VolumeTypeScratch)))
		}
	}
	return allErrs
}

func validateHyperDisk(dataVolume core.DataVolume, config gcp.DataVolume) field.ErrorList {
	allErrs := field.ErrorList{}

	if config.ProvisionedIops != nil && !slices.Contains(worker.AllowedTypesIops, *dataVolume.Type) {
		allErrs = append(allErrs, field.Forbidden(
			dataVolumeFldPath.Child("provisionedIops"),
			fmt.Sprintf("is only allowed for types: %v", worker.AllowedTypesIops)))
	}
	if config.ProvisionedThroughput != nil && !slices.Contains(worker.AllowedTypesThroughput, *dataVolume.Type) {
		allErrs = append(allErrs, field.Forbidden(
			dataVolumeFldPath.Child("provisionedThroughput"),
			fmt.Sprintf("is only allowed for types: %v", worker.AllowedTypesThroughput)))
	}
	return allErrs
}

func validateDataVolumeConfigs(dataVolumes []core.DataVolume, configs []gcp.DataVolume) field.ErrorList {
	allErrs := field.ErrorList{}

	volumeNames := sets.New[string]()
	for i, configDataVolume := range configs {
		idx := slices.IndexFunc(dataVolumes, func(dv core.DataVolume) bool { return dv.Name == configDataVolume.Name })
		volumeName := configDataVolume.Name
		if idx == -1 {
			allErrs = append(allErrs, field.Invalid(
				dataVolumeFldPath,
				volumeName,
				fmt.Sprintf("could not find dataVolume with name %s", volumeName)))
			continue
		}
		if volumeNames.Has(volumeName) {
			allErrs = append(allErrs, field.Duplicate(dataVolumeFldPath.Index(i), volumeName))
			continue
		}
		volumeNames.Insert(volumeName)
		allErrs = append(allErrs, validateHyperDisk(dataVolumes[idx], configDataVolume)...)
	}

	return allErrs
}

func validateNodeTemplate(nt *extensionsv1alpha1.NodeTemplate, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if nt == nil {
		return allErrs
	}

	if len(nt.Capacity) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("capacity"), "capacity must not be empty"))
	}

	for _, capacityAttribute := range []corev1.ResourceName{"cpu", "gpu", "memory"} {
		value, ok := nt.Capacity[capacityAttribute]
		if !ok {
			// resources such as "cpu", "gpu", "memory" need not be explicitly specified in workerConfig.NodeTemplate.
			// Will use the workerConfig.gpu.count or worker pool's node template gpu.
			continue
		}
		allErrs = append(allErrs, validateResourceQuantityValue(capacityAttribute, value, fldPath.Child("capacity").Child(string(capacityAttribute)))...)
	}

	return allErrs
}

func validateResourceQuantityValue(key corev1.ResourceName, value resource.Quantity, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if value.Cmp(resource.Quantity{}) < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, value.String(), fmt.Sprintf("%s value must not be negative", key)))
	}

	return allErrs
}
