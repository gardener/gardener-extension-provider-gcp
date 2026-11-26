// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/oauth2/google"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// validateTokenURI checks that token_uri is one of the known Google token endpoints.
// Pinning this prevents tenants from redirecting seed-side token requests to arbitrary hosts (SSRF).
func validateTokenURI(urlStr string, fldPath *field.Path) field.ErrorList {
	// Inline to prevent package-level mutation by tests.
	allowed := []string{
		google.JWTTokenURL,                           // https://oauth2.googleapis.com/token
		google.MTLSTokenURL,                          // https://oauth2.mtls.googleapis.com/token
		"https://accounts.google.com/o/oauth2/token", // no library constant; issued by Google Cloud Console
	}

	for _, a := range allowed {
		if urlStr == a {
			return field.ErrorList{}
		}
	}

	return field.ErrorList{field.Invalid(fldPath, urlStr,
		fmt.Sprintf("must be one of the allowed Google token endpoints: %s", strings.Join(allowed, ", ")))}
}

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
