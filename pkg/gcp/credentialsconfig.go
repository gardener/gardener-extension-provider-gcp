// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type credConfig struct {
	ProjectID string `json:"project_id"`
	Email     string `json:"client_email"`
	Type      string `json:"type"`

	Audience         string           `json:"audience"`
	CredentialSource credentialSource `json:"credential_source"`
	UniverseDomain   string           `json:"universe_domain"`
	TokenURL         string           `json:"token_url"`
	SubjectTokenType string           `json:"subject_token_type"`
}

type credentialSource struct {
	File   string            `json:"file"`
	URL    string            `json:"url"`
	Format credentialsFormat `json:"format"`
}

type credentialsFormat struct {
	Type string `json:"type"`
}

// CredentialsConfig represents a GCP credentials configuration.
type CredentialsConfig struct {
	// Raw is the raw representation of the GCP credentials config.
	Raw []byte
	// ProjectID is the project id the credentials are associated to.
	ProjectID string
	// Email is the email associated with the service account.
	Email string
	// Type is the type of credentials.
	Type string

	// The following fields are only used when the credentials configuration is of type "external_account".

	// TokenFilePath is file path which stores the token used for authentication.
	TokenFilePath string

	// TokenRequestURL is the URL which will be queried to retrieve the external account's token.
	TokenRequestURL string
	// TokenRequestClient is the client which will be used to query the TokenRequestURL.
	TokenRequestClient *http.Client
}

// GetCredentialsConfigFromSecretReference retrieves the credentials config from the secret with the given secret reference.
func GetCredentialsConfigFromSecretReference(ctx context.Context, c client.Client, secretRef corev1.SecretReference) (*CredentialsConfig, error) {
	secret, err := extensionscontroller.GetSecretByReference(ctx, c, &secretRef)
	if err != nil {
		return nil, err
	}

	return GetCredentialsConfigFromSecret(secret)
}

// GetCredentialsConfigFromSecret retrieves the credentials config from the secret.
func GetCredentialsConfigFromSecret(secret *corev1.Secret) (*CredentialsConfig, error) {
	if data, ok := secret.Data[ServiceAccountJSONField]; ok {
		credentialsConfig, err := GetCredentialsConfigFromJSON(data)
		if err != nil {
			return nil, fmt.Errorf("could not get credentials config from %q field: %w", ServiceAccountJSONField, err)
		}
		if credentialsConfig.ProjectID == "" {
			credentialsConfig.ProjectID = string(secret.Data["projectID"])
		}
		return credentialsConfig, nil
	}

	if data, ok := secret.Data[CredentialsConfigField]; ok {
		credentialsConfig, err := GetCredentialsConfigFromJSON(data)
		if err != nil {
			return nil, fmt.Errorf("could not get credentials config from %q field: %w", CredentialsConfigField, err)
		}
		if credentialsConfig.ProjectID == "" {
			credentialsConfig.ProjectID = string(secret.Data["projectID"])
		}
		return credentialsConfig, nil
	}

	return nil, fmt.Errorf("secret %s doesn't have a credentials config json (expected field: %q or %q)", client.ObjectKeyFromObject(secret), ServiceAccountJSONField, CredentialsConfigField)
}

// GetCredentialsConfigFromJSON returns a credentials config from the given data.
func GetCredentialsConfigFromJSON(data []byte) (*CredentialsConfig, error) {
	var credentialsConfig credConfig
	if err := json.Unmarshal(data, &credentialsConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal json object: %w", err)
	}

	if credentialsConfig.Type == "service_account" && credentialsConfig.ProjectID == "" {
		return nil, fmt.Errorf("no project id specified")
	}

	return &CredentialsConfig{
		Raw:             data,
		ProjectID:       credentialsConfig.ProjectID,
		Email:           credentialsConfig.Email,
		Type:            credentialsConfig.Type,
		TokenFilePath:   credentialsConfig.CredentialSource.File,
		TokenRequestURL: credentialsConfig.CredentialSource.URL,
	}, nil
}

func (c *CredentialsConfig) InjectURLCredentialSource(url string, client *http.Client) (bool, error) {
	if c.Type != ExternalAccountCredentialType {
		return false, nil
	}
	var credentialsConfig credConfig
	if err := json.Unmarshal(c.Raw, &credentialsConfig); err != nil {
		return false, fmt.Errorf("failed to unmarshal json object: %w", err)
	}

	credentialsConfig.CredentialSource = credentialSource{
		URL: url,
		Format: credentialsFormat{
			Type: "text",
		},
	}

	if newRawConfig, err := json.Marshal(&credentialsConfig); err != nil {
		return false, fmt.Errorf("failed to marshal object: %w", err)
	} else {
		newConfig := &CredentialsConfig{
			Raw:                newRawConfig,
			ProjectID:          c.ProjectID,
			Email:              c.Email,
			Type:               c.Type,
			TokenRequestURL:    credentialsConfig.CredentialSource.URL,
			TokenRequestClient: client,
		}
		*c = *newConfig
		return true, nil
	}
}
