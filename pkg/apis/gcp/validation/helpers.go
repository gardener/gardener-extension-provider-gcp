// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"net/url"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

// validateHTTPSURL validates that a string is a valid HTTPS URL
func validateHTTPSURL(urlStr string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if urlStr == "" {
		allErrs = append(allErrs, field.Required(fldPath, "URL cannot be empty if key is set"))
		return allErrs
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, urlStr, "must be a valid URL"))
		return allErrs
	}

	if parsedURL.Scheme != "https" {
		allErrs = append(allErrs, field.Invalid(fldPath, urlStr, "must use https:// scheme"))
	}

	return allErrs
}
