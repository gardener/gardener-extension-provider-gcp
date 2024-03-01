// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"regexp"

	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

var projectIDRegexp = regexp.MustCompile(`^(?P<project>[a-z][a-z0-9-]{4,28}[a-z0-9])$`)

// ValidateCloudProviderSecret checks whether the given secret contains a valid GCP service account.
func ValidateCloudProviderSecret(secret *corev1.Secret) error {
	serviceAccountJSON, ok := secret.Data[gcp.ServiceAccountJSONField]
	if !ok {
		return fmt.Errorf("missing %q field in secret", gcp.ServiceAccountJSONField)
	}

	sa, err := gcp.GetServiceAccountFromJSON(serviceAccountJSON)
	if err != nil {
		return err
	}

	if sa.Type != gcp.ServiceAccountCredentialType {
		return fmt.Errorf("forbidden credential type %q used. Only %q is allowed", sa.Type, gcp.ServiceAccountCredentialType)
	}

	if !projectIDRegexp.MatchString(sa.ProjectID) {
		return fmt.Errorf("service account project ID does not match the expected format '%s'", projectIDRegexp)
	}

	return nil
}
