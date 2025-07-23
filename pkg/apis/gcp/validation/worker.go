// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
)

// ValidateWorkerConfig validates a WorkerConfig object.
func ValidateWorkerConfig(workerConfig gcp.WorkerConfig, dataVolumes []core.DataVolume, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for idx, dataVolume := range dataVolumes {
		if dataVolume.Type == nil {
			allErrs = append(allErrs, field.Required(field.NewPath("dataVolume").Index(idx).Child("type"), "must not be empty"))
			return allErrs
		}

		allErrs = append(allErrs, validateScratchDisk(*dataVolume.Type, workerConfig, fldPath.Child("volume"))...)
	}

	allErrs = append(allErrs, validateGPU(workerConfig.GPU, fldPath.Child("gpu"))...)
	if workerConfig.ServiceAccount != nil {
		allErrs = append(allErrs, validateServiceAccount(*workerConfig.ServiceAccount, fldPath.Child("serviceAccount"))...)
	}
	if workerConfig.Volume != nil {
		allErrs = append(allErrs, validateVolumeConfig(*workerConfig.Volume, fldPath.Child("volume"))...)
	}
	allErrs = append(allErrs, validateNodeTemplate(workerConfig.NodeTemplate, fldPath.Child("nodeTemplate"))...)
	if workerConfig.DataVolumes != nil {
		allErrs = append(allErrs, validateDataVolumeConfigs(dataVolumes, workerConfig.DataVolumes, fldPath.Child("dataVolumes"))...)
	}

	if workerConfig.MinCpuPlatform != nil {
		allErrs = append(allErrs, validateMinCPUsPlatform(*workerConfig.MinCpuPlatform, fldPath.Child("minCpuPlatform"))...)
	}

	return allErrs
}

func validateGPU(gpu *gcp.GPU, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if gpu == nil {
		return allErrs
	}

	allErrs = append(allErrs, validateGpuAcceleratorType(gpu.AcceleratorType, fldPath.Child("acceleratorType"))...)

	if gpu.Count <= 0 {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("count"), "must be > 0 when providing gpu"))
	}

	return allErrs
}

func validateServiceAccount(sa gcp.ServiceAccount, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateServiceAccountEmail(sa.Email, fldPath.Child("email"))...)

	if len(sa.Scopes) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("scopes"), "must have at least one scope"))
	} else {
		existingScopes := sets.NewString()

		for i, scope := range sa.Scopes {
			idxPath := fldPath.Child("scopes").Index(i)

			allErrs = append(allErrs, validateServiceAccountScopeName(scope, idxPath)...)

			switch {
			case existingScopes.Has(scope):
				allErrs = append(allErrs, field.Duplicate(idxPath, scope))
			default:
				existingScopes.Insert(scope)
			}
		}
	}

	return allErrs
}

func validateVolumeConfig(volume gcp.Volume, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if volume.LocalSSDInterface != nil {
		if err := validVolumeLocalSSDInterfacesTypes.Has(*volume.LocalSSDInterface); !err {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("interface"),
				*volume.LocalSSDInterface, validVolumeLocalSSDInterfacesTypes.UnsortedList()))
		}
	}

	if volume.Encryption != nil {
		allErrs = append(allErrs, validateDiskEncryption(*volume.Encryption, fldPath.Child("encryption"))...)
	}

	return allErrs
}

// validateDiskEncryption validates the provider specific disk encryption configuration for a volume
func validateDiskEncryption(encryption gcp.DiskEncryption, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if encryption.KmsKeyName == nil || strings.TrimSpace(*encryption.KmsKeyName) == "" {
		// Currently DiskEncryption only contains CMEK fields. Hence if not nil, then kmsKeyName is a must
		// Validation logic will need to be modified when CSEK fields are possibly added to gcp.DiskEncryption in the future.
		allErrs = append(allErrs, field.Required(fldPath.Child("kmsKeyName"), "must be specified when configuring disk encryption"))
		return allErrs
	}

	allErrs = append(allErrs, validateVolumeKmsKeyName(*encryption.KmsKeyName, fldPath.Child("kmsKeyName"))...)

	if encryption.KmsKeyServiceAccount != nil {
		allErrs = append(allErrs, validateVolumeKmsKeyServiceAccount(*encryption.KmsKeyServiceAccount, fldPath.Child("kmsKeyServiceAccount"))...)
	}

	return allErrs
}

func validateScratchDisk(volumeType string, workerConfig gcp.WorkerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	interfacePath := fldPath.Child("interface")
	encryptionPath := fldPath.Child("encryption")

	if volumeType == worker.VolumeTypeScratch {
		if workerConfig.Volume == nil || workerConfig.Volume.LocalSSDInterface == nil {
			allErrs = append(allErrs, field.Required(interfacePath, fmt.Sprintf("must be set when using %s volumes", worker.VolumeTypeScratch)))
		}
		// DiskEncryption not allowed for type SCRATCH
		if workerConfig.Volume != nil && workerConfig.Volume.Encryption != nil {
			allErrs = append(allErrs, field.Invalid(encryptionPath, *workerConfig.Volume.Encryption, fmt.Sprintf("must not be set in combination with %s volumes", worker.VolumeTypeScratch)))
		}
	} else {
		// LocalSSDInterface only allowed for type SCRATCH
		if workerConfig.Volume != nil && workerConfig.Volume.LocalSSDInterface != nil {
			allErrs = append(allErrs, field.Invalid(encryptionPath, *workerConfig.Volume.LocalSSDInterface, fmt.Sprintf("is only allowed for type %s", worker.VolumeTypeScratch)))
		}
	}
	return allErrs
}

func validateDataVolumeConfigs(dataVolumes []core.DataVolume, configs []gcp.DataVolume, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	volumeNames := sets.New[string]()

	for i, configDataVolume := range configs {
		if sourceImage := configDataVolume.SourceImage; sourceImage != nil {
			allErrs = append(allErrs, validateVolumeSourceImage(*sourceImage, fldPath.Index(i).Child("sourceImage"))...)
		}
		idx := slices.IndexFunc(dataVolumes, func(dv core.DataVolume) bool { return dv.Name == configDataVolume.Name })
		volumeName := configDataVolume.Name
		if idx == -1 {
			allErrs = append(allErrs, field.Invalid(fldPath, volumeName,
				fmt.Sprintf("could not find dataVolume with name %s", volumeName)))
			continue
		}
		if volumeNames.Has(volumeName) {
			allErrs = append(allErrs, field.Duplicate(fldPath.Index(i), volumeName))
			continue
		}
		volumeNames.Insert(volumeName)
		allErrs = append(allErrs, validateHyperDisk(dataVolumes[idx], configDataVolume, fldPath)...)
	}

	return allErrs
}

func validateHyperDisk(dataVolume core.DataVolume, config gcp.DataVolume, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if config.ProvisionedIops != nil && !slices.Contains(worker.AllowedTypesIops, *dataVolume.Type) {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("provisionedIops"),
			fmt.Sprintf("is only allowed for types: %v", worker.AllowedTypesIops)))
	}
	if config.ProvisionedThroughput != nil && !slices.Contains(worker.AllowedTypesThroughput, *dataVolume.Type) {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("provisionedThroughput"),
			fmt.Sprintf("is only allowed for types: %v", worker.AllowedTypesThroughput)))
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
