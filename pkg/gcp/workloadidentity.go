// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"

	securityv1alpha1constants "github.com/gardener/gardener/pkg/apis/security/v1alpha1/constants"
	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
)

var (
	usedCredentialsConfigFields = map[string]struct{}{
		"universe_domain":                   {},
		"type":                              {},
		"audience":                          {},
		"subject_token_type":                {},
		"token_url":                         {},
		"service_account_impersonation_url": {},
	}
)

// SetWorkloadIdentityFeatures sets in-place the WorkloadIdentity features
// that can ease up creation of GCP clients. The provided map must contain valid
// WorkloadIdentityConfig under the `config` data key, which is used to populate
// values for the keys `credentialsConfig` and `projectID`.
func SetWorkloadIdentityFeatures(data map[string][]byte, tokenMountDir string) error {
	if _, ok := data[securityv1alpha1constants.DataKeyConfig]; !ok {
		return errors.New("'config' key is missing in the map")
	}

	workloadIdentityConfig, err := helper.WorkloadIdentityConfigFromBytes(data[securityv1alpha1constants.DataKeyConfig])
	if err != nil {
		return fmt.Errorf("could not decode 'config' as WorkloadIdentityConfig: %w", err)
	}

	config := map[string]any{}
	if err := json.Unmarshal(workloadIdentityConfig.CredentialsConfig.Raw, &config); err != nil {
		return fmt.Errorf("could not unmarshal credential config: %w", err)
	}

	maps.DeleteFunc(config, func(k string, _ any) bool {
		_, contain := usedCredentialsConfigFields[k]
		return !contain
	})

	config["credential_source"] = map[string]any{
		"file": tokenMountDir + "/token",
		"format": map[string]string{
			"type": "text",
		},
	}
	newConfig, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("could not marshal new config: %w", err)
	}

	data[CredentialsConfigField] = newConfig
	data[ProjectIDField] = []byte(workloadIdentityConfig.ProjectID)
	return nil
}

// IsWorkloadIdentitySecret checks if the secret has the required features to be
// classified as secret bearing GCP workload identity tokens.
func IsWorkloadIdentitySecret(secret *corev1.Secret) bool {
	if secret.Labels == nil {
		return false
	}

	if secret.Labels[securityv1alpha1constants.LabelPurpose] != securityv1alpha1constants.LabelPurposeWorkloadIdentityTokenRequestor {
		return false
	}

	if secret.Labels[securityv1alpha1constants.LabelWorkloadIdentityProvider] != Type {
		return false
	}

	return true
}
