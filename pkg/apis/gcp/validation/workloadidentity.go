// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

const (
	keyUniverseDomain   = "universe_domain"
	keyAudience         = "audience"
	keyType             = "type"
	keyTokenURL         = "token_url"
	keySubjectTokenType = "subject_token_type"
)

var (
	usedCredentialsConfigFields = map[string]struct{}{
		keyUniverseDomain:   {},
		keyType:             {},
		keyAudience:         {},
		keySubjectTokenType: {},
		keyTokenURL:         {},
	}
)

// ValidateWorkloadIdentityConfig checks whether the given workload identity configuration contains expected fields and values.
func ValidateWorkloadIdentityConfig(config *apisgcp.WorkloadIdentityConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if !projectIDRegexp.MatchString(config.ProjectID) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("projectID"), config.ProjectID, "does not match the expected format"))
	}

	if config.CredentialsConfig == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("credentialsConfig"), "is required"))
	}

	cfg := map[string]any{}
	if err := json.Unmarshal(config.CredentialsConfig.Raw, &cfg); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("credentialsConfig"), config.CredentialsConfig.Raw, "has invalid format"))
	} else {
		// we do not care about this field since it will be overwritten anyways
		delete(cfg, "credential_source")

		cloned := maps.Clone(cfg)
		for f := range usedCredentialsConfigFields {
			delete(cloned, f)
		}
		if len(cloned) != 0 {
			requiredFields := slices.Collect(maps.Keys(usedCredentialsConfigFields))
			slices.Sort(requiredFields)
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("credentialsConfig"), "contains extra fields, required fields are: "+strings.Join(requiredFields, ", ")))
		}

		for f := range usedCredentialsConfigFields {
			if _, ok := cfg[f]; !ok {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("credentialsConfig"), fmt.Sprintf("missing required field: %q", f)))
			}
		}

		if cfg[keyType] != gcp.ExternalAccountCredentialType {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("credentialsConfig").Child(keyType), cfg[keyType], fmt.Sprintf("should equal %q", gcp.ExternalAccountCredentialType)))
		}

		if cfg[keySubjectTokenType] != "urn:ietf:params:oauth:token-type:jwt" {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("credentialsConfig").Child(keySubjectTokenType), cfg[keySubjectTokenType], fmt.Sprintf("should equal %q", "urn:ietf:params:oauth:token-type:jwt")))
		}

		retrievedURL := cfg[keyTokenURL]
		rawURL, ok := retrievedURL.(string)
		if !ok {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("credentialsConfig").Child(keyTokenURL), cfg[keyTokenURL], "should be string"))
		}
		tokenURL, err := url.Parse(rawURL)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("credentialsConfig").Child(keyTokenURL), cfg[keyTokenURL], "should be a valid URL"))
		}

		if tokenURL.Scheme != "https" {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("credentialsConfig").Child(keyTokenURL), cfg[keyTokenURL], "should start with https://"))
		}
	}

	return allErrs
}

// ValidateWorkloadIdentityConfigUpdate validates updates on WorkloadIdentityConfig object.
func ValidateWorkloadIdentityConfigUpdate(oldConfig, newConfig *apisgcp.WorkloadIdentityConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newConfig.ProjectID, oldConfig.ProjectID, fldPath.Child("projectID"))...)
	allErrs = append(allErrs, ValidateWorkloadIdentityConfig(newConfig, fldPath)...)

	return allErrs
}
