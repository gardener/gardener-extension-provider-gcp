// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"regexp"
	"unicode/utf8"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

var (
	// Rfc1035Regex matches strings that comply with RFC1035.
	Rfc1035Regex = `^[a-z]([-a-z0-9]*[a-z0-9])?$`
	// GpuAcceleratorTypeRegex matches e.g. nvidia-tesla-p100
	GpuAcceleratorTypeRegex = `^[a-z0-9\-]+$`
	// MinCPUsPlatformRegex matches e.g. "Intel Haswell" or "Intel Sandy Bridge"
	MinCPUsPlatformRegex = `^[A-Za-z0-9\s]+$`
	// VolumeSourceImageRegex matches e.g. family/debian-9 or projects/debian-cloud/global/images/family/debian-9
	VolumeSourceImageRegex = `^[a-z0-9\-/]+$`
	// VolumeKmsKeyNameRegex matches e.g. projects/projectId/locations/<zoneName>/keyRings/<keyRingName>/cryptoKeys/alpha
	VolumeKmsKeyNameRegex = `^[a-zA-Z0-9\-_/]+$`
	// ServiceAccountRegex matches e.g. user@projectId.iam.gserviceaccount.com
	ServiceAccountRegex = `^[a-zA-Z0-9\-_]+@[a-z0-9\-]+\.iam\.gserviceaccount\.com$`
	// ServiceAccountScopeRegex matches e.g.https://www.googleapis.com/auth/cloud-platform
	ServiceAccountScopeRegex = `^https:\/\/www\.googleapis\.com\/[a-z0-9\-\.\/_]+$`

	validateGcpResourceName            = combineValidationFuncs(regex(Rfc1035Regex), minLength(1), maxLength(63))
	validateGpuAcceleratorType         = combineValidationFuncs(regex(GpuAcceleratorTypeRegex), minLength(1), maxLength(250))
	validateMinCPUsPlatform            = combineValidationFuncs(regex(MinCPUsPlatformRegex), minLength(1), maxLength(250))
	validateVolumeSourceImage          = combineValidationFuncs(regex(VolumeSourceImageRegex), minLength(1), maxLength(250))
	validateVolumeKmsKeyName           = combineValidationFuncs(regex(VolumeKmsKeyNameRegex), minLength(1), maxLength(250))
	validateVolumeKmsKeyServiceAccount = combineValidationFuncs(regex(ServiceAccountRegex), minLength(1), maxLength(250))
	validateServiceAccountEmail        = combineValidationFuncs(regex(ServiceAccountRegex), minLength(1), maxLength(250))
	validateServiceAccountScopeName    = combineValidationFuncs(regex(ServiceAccountScopeRegex), minLength(1), maxLength(250))
)

type validateFunc[T any] func(T, *field.Path) field.ErrorList

// combineValidationFuncs validates a value against a list of filters.
func combineValidationFuncs[T any](filters ...validateFunc[T]) validateFunc[T] {
	return func(t T, fld *field.Path) field.ErrorList {
		var allErrs field.ErrorList
		for _, f := range filters {
			allErrs = append(allErrs, f(t, fld)...)
		}
		return allErrs
	}
}

// regex returns a filterFunc that validates a string against a regular expression.
func regex(regex string) validateFunc[string] {
	compiled := regexp.MustCompile(regex)
	return func(name string, fld *field.Path) field.ErrorList {
		var allErrs field.ErrorList
		if name == "" {
			return allErrs // Allow empty strings to pass through
		}
		if !compiled.MatchString(name) {
			allErrs = append(allErrs, field.Invalid(fld, name, fmt.Sprintf("does not match expected regex %s", compiled.String())))
		}
		return allErrs
	}
}

// nolint:unparam
func minLength(min int) validateFunc[string] {
	return func(name string, fld *field.Path) field.ErrorList {
		var allErrs field.ErrorList
		if utf8.RuneCountInString(name) < min {
			return field.ErrorList{field.Invalid(fld, name, fmt.Sprintf("must not be fewer than %d characters, got %d", min, len(name)))}
		}
		return allErrs
	}
}

func maxLength(max int) validateFunc[string] {
	return func(name string, fld *field.Path) field.ErrorList {
		var allErrs field.ErrorList
		if utf8.RuneCountInString(name) > max {
			return field.ErrorList{field.Invalid(fld, name, fmt.Sprintf("must not be more than %d characters, got %d", max, len(name)))}
		}
		return allErrs
	}
}
