// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

var projectIDRegexp = regexp.MustCompile(`^(?P<project>[a-z][a-z0-9-]{4,28}[a-z0-9])$`)

var serviceAccountAllowedFields = map[string]struct{}{
	"type":                        {},
	"project_id":                  {},
	"client_email":                {},
	"universe_domain":             {},
	"auth_uri":                    {},
	"auth_provider_x509_cert_url": {},
	"client_x509_cert_url":        {},
	"client_id":                   {},
	"private_key_id":              {},
	"private_key":                 {},
	"token_uri":                   {},
}

// ValidateCloudProviderSecret checks whether the given secret contains a valid GCP service account.
func ValidateCloudProviderSecret(secret *corev1.Secret) error {
	serviceAccountJSON, ok := secret.Data[gcp.ServiceAccountJSONField]
	if !ok {
		return fmt.Errorf("missing %q field in secret", gcp.ServiceAccountJSONField)
	}

	fields := map[string]string{}
	if err := json.Unmarshal(serviceAccountJSON, &fields); err != nil {
		return fmt.Errorf("failed to unmarshal 'serviceaccount.json' field: %w", err)
	}

	for f := range fields {
		if _, ok := serviceAccountAllowedFields[f]; !ok {
			return fmt.Errorf("forbidden fields are present. Allowed fields are %s", strings.Join(slices.Collect(maps.Keys(serviceAccountAllowedFields)), ", "))
		}
	}

	sa, err := gcp.GetCredentialsConfigFromJSON(serviceAccountJSON)
	if err != nil {
		return fmt.Errorf("could not get service account from %q field: %w", gcp.ServiceAccountJSONField, err)
	}

	if sa.Type != gcp.ServiceAccountCredentialType {
		return fmt.Errorf("forbidden credential type %q used. Only %q is allowed", sa.Type, gcp.ServiceAccountCredentialType)
	}

	if !projectIDRegexp.MatchString(sa.ProjectID) {
		return fmt.Errorf("service account project ID does not match the expected format '%s'", projectIDRegexp)
	}

	return nil
}
