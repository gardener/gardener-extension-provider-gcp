// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"github.com/gardener/gardener/pkg/apis/core"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
)

var validVolumeLocalSSDInterfacesTypes = sets.New("NVME", "SCSI")

// ValidateWorkerConfig validates a WorkerConfig object.
func ValidateWorkerConfig(workerConfig *gcp.WorkerConfig, dataVolumes []core.DataVolume) field.ErrorList {
	allErrs := field.ErrorList{}

	for _, volume := range dataVolumes {
		if volume.Type != nil && *volume.Type == "SCRATCH" {
			if workerConfig == nil || workerConfig.Volume == nil || workerConfig.Volume.LocalSSDInterface == nil {
				allErrs = append(allErrs, field.Required(field.NewPath("volume", "localSSDInterface"), "must be set when using SCRATCH volumes"))
			} else {
				if !validVolumeLocalSSDInterfacesTypes.Has(*workerConfig.Volume.LocalSSDInterface) {
					allErrs = append(allErrs, field.NotSupported(field.NewPath("volume", "localSSDInterface"), *workerConfig.Volume.LocalSSDInterface, validVolumeLocalSSDInterfacesTypes.UnsortedList()))
				}
			}
		}
	}

	if workerConfig != nil {
		allErrs = append(allErrs, validateGPU(workerConfig.GPU, field.NewPath("gpu"))...)
		allErrs = append(allErrs, validateServiceAccount(workerConfig.ServiceAccount, field.NewPath("serviceAccount"))...)
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
