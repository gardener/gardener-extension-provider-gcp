// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

// serviceAccountRequiredFields defines the required fields in a GCP service account JSON
// based on what JWTConfigFromJSON from golang.org/x/oauth2/google actually uses
var serviceAccountRequiredFields = sets.New(
	"type",
	"project_id",
	"client_email",
	"private_key_id",
	"private_key",
	"token_uri",
)

// serviceAccountOptionalFields defines optional fields in a GCP service account JSON
var serviceAccountOptionalFields = sets.New(
	"client_id",
	"auth_uri",
	"auth_provider_x509_cert_url",
	"client_x509_cert_url",
	"universe_domain",
)

// ValidateCloudProviderSecret validates GCP service account credentials
func ValidateCloudProviderSecret(secret *corev1.Secret, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	secretKey := fmt.Sprintf("%s/%s", secret.Namespace, secret.Name)
	dataPath := fldPath.Child("data")

	// Ensure that secret contains exactly one field: serviceaccount.json
	if len(secret.Data) != 1 {
		allErrs = append(allErrs, field.Invalid(dataPath, len(secret.Data),
			fmt.Sprintf("secret %s must contain exactly one field %q, got %d fields", secretKey, gcp.ServiceAccountJSONField, len(secret.Data))))
	}

	// Ensure serviceaccount.json field exists and is non-empty
	serviceAccountJSON, ok := secret.Data[gcp.ServiceAccountJSONField]
	if !ok {
		allErrs = append(allErrs, field.Required(dataPath.Key(gcp.ServiceAccountJSONField),
			fmt.Sprintf("missing required field %q in secret %s", gcp.ServiceAccountJSONField, secretKey)))
		return allErrs
	}

	if len(serviceAccountJSON) == 0 {
		allErrs = append(allErrs, field.Invalid(dataPath.Key(gcp.ServiceAccountJSONField), "",
			fmt.Sprintf("field %q cannot be empty in secret %s", gcp.ServiceAccountJSONField, secretKey)))
		return allErrs
	}

	// Parse JSON
	var fields map[string]string
	if err := json.Unmarshal(serviceAccountJSON, &fields); err != nil {
		allErrs = append(allErrs, field.Invalid(dataPath.Key(gcp.ServiceAccountJSONField), "(hidden)",
			fmt.Sprintf("field %q must be valid JSON in secret %s: %v", gcp.ServiceAccountJSONField, secretKey, err)))
		return allErrs
	}

	saFieldPath := dataPath.Key(gcp.ServiceAccountJSONField)

	// Validate all fields
	allErrs = append(allErrs, validateServiceAccountFields(fields, saFieldPath, secretKey)...)

	return allErrs
}

// validateServiceAccountFields validates the structure and content of service account JSON fields
func validateServiceAccountFields(fields map[string]string, fldPath *field.Path, secretKey string) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate required fields are present and non-empty
	for requiredField := range serviceAccountRequiredFields {
		value, exists := fields[requiredField]
		if !exists {
			allErrs = append(allErrs, field.Required(fldPath.Child(requiredField),
				fmt.Sprintf("missing required field %q in service account JSON in secret %s", requiredField, secretKey)))
			continue
		}
		if value == "" {
			allErrs = append(allErrs, field.Invalid(fldPath.Child(requiredField), "",
				fmt.Sprintf("field %q cannot be empty in service account JSON in secret %s", requiredField, secretKey)))
		}
	}

	// Validate no unexpected fields
	for fieldName := range fields {
		if !serviceAccountRequiredFields.Has(fieldName) && !serviceAccountOptionalFields.Has(fieldName) {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child(fieldName),
				fmt.Sprintf("unexpected field %q in service account JSON in secret %s", fieldName, secretKey)))
		}
	}

	// Validate field formats
	allErrs = append(allErrs, validateFieldFormats(fields, fldPath)...)

	return allErrs
}

// validateFieldFormats validates the format of specific service account fields
func validateFieldFormats(fields map[string]string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate type field
	if typeVal, ok := fields["type"]; ok {
		if typeVal != gcp.ServiceAccountCredentialType {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"), typeVal, []string{gcp.ServiceAccountCredentialType}))
		}
	}

	// Validate project_id format
	if projectID, ok := fields["project_id"]; ok && projectID != "" {
		allErrs = append(allErrs, validateProjectID(projectID, fldPath.Child("project_id"))...)
	}

	// Validate client_email format
	if clientEmail, ok := fields["client_email"]; ok && clientEmail != "" {
		allErrs = append(allErrs, validateServiceAccountEmail(clientEmail, fldPath.Child("client_email"))...)
	}

	// Validate client_id format
	if clientID, ok := fields["client_id"]; ok && clientID != "" {
		allErrs = append(allErrs, validateClientID(clientID, fldPath.Child("client_id"))...)
	}

	// Validate private_key_id format
	if privateKeyID, ok := fields["private_key_id"]; ok && privateKeyID != "" {
		allErrs = append(allErrs, validatePrivateKeyID(privateKeyID, fldPath.Child("private_key_id"))...)
	}

	// Validate private_key format
	if privateKey, ok := fields["private_key"]; ok && privateKey != "" {
		allErrs = append(allErrs, validatePEMPrivateKey(privateKey, fldPath.Child("private_key"))...)
	}

	// Validate URL fields
	urlFields := []string{
		"auth_uri",
		"token_uri",
		"auth_provider_x509_cert_url",
		"client_x509_cert_url",
	}

	for _, fieldName := range urlFields {
		if urlValue, ok := fields[fieldName]; ok && urlValue != "" {
			allErrs = append(allErrs, validateHTTPSURL(urlValue, fldPath.Child(fieldName))...)
		}
	}

	return allErrs
}

// validatePEMPrivateKey validates that a string is a valid PEM-encoded private key
func validatePEMPrivateKey(key string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	const (
		pemHeader = "-----BEGIN PRIVATE KEY-----"
		pemFooter = "-----END PRIVATE KEY-----\n"
	)

	if !strings.HasPrefix(key, pemHeader) {
		allErrs = append(allErrs, field.Invalid(fldPath, "(hidden)", "must start with '-----BEGIN PRIVATE KEY-----'"))
	}

	if !strings.HasSuffix(key, pemFooter) {
		allErrs = append(allErrs, field.Invalid(fldPath, "(hidden)", "must end with '-----END PRIVATE KEY-----'"))
	}

	return allErrs
}
