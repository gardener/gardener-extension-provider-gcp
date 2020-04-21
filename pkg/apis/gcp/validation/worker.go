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
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var validVolumeLocalSSDInterfacesTypes = sets.NewString("NVME", "SCSI")

// ValidateWorkerConfig validates a WorkerConfig object.
func ValidateWorkerConfig(workerConfig *gcp.WorkerConfig, volumeType *string) field.ErrorList {
	allErrs := field.ErrorList{}

	if volumeType != nil && *volumeType == "SCRATCH" {
		if workerConfig == nil || workerConfig.Volume == nil || workerConfig.Volume.LocalSSDInterface == nil {
			allErrs = append(allErrs, field.Required(field.NewPath("volume", "localSSDInterface"), "must be set when using SCRATCH volumes"))
		} else {
			if !validVolumeLocalSSDInterfacesTypes.Has(*workerConfig.Volume.LocalSSDInterface) {
				allErrs = append(allErrs, field.NotSupported(field.NewPath("volume", "localSSDInterface"), *workerConfig.Volume.LocalSSDInterface, validVolumeLocalSSDInterfacesTypes.List()))
			}
		}
	}

	return allErrs
}
