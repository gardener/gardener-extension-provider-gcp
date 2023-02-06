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
	"fmt"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/strings/slices"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
)

// ValidateCloudProfileConfig validates a CloudProfileConfig object.
func ValidateCloudProfileConfig(cpConfig *apisgcp.CloudProfileConfig, machineImages []core.MachineImage, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	machineImagesPath := fldPath.Child("machineImages")

	for _, image := range machineImages {
		var processed bool
		for i, imageConfig := range cpConfig.MachineImages {
			if image.Name == imageConfig.Name {
				allErrs = append(allErrs, validateVersions(imageConfig.Versions, helper.ToExpirableVersions(image.Versions), machineImagesPath.Index(i).Child("versions"))...)
				processed = true
				break
			}
		}
		if !processed && len(image.Versions) > 0 {
			allErrs = append(allErrs, field.Required(machineImagesPath, fmt.Sprintf("must provide an image mapping for image %q", image.Name)))
		}
	}

	return allErrs
}

func validateVersions(versionsConfig []apisgcp.MachineImageVersion, versions []core.ExpirableVersion, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for _, version := range versions {
		var processed bool
		for j, versionConfig := range versionsConfig {
			jdxPath := fldPath.Index(j)
			if version.Version == versionConfig.Version {
				if len(versionConfig.Image) == 0 {
					allErrs = append(allErrs, field.Required(jdxPath.Child("image"), "must provide an image"))
				}
				if !slices.Contains(v1beta1constants.ValidArchitectures, *versionConfig.Architecture) {
					allErrs = append(allErrs, field.NotSupported(jdxPath.Child("architecture"), *versionConfig.Architecture, v1beta1constants.ValidArchitectures))
				}
				processed = true
				break
			}
		}
		if !processed {
			allErrs = append(allErrs, field.Required(fldPath, fmt.Sprintf("must provide an image mapping for version %q", version.Version)))
		}
	}

	return allErrs
}
