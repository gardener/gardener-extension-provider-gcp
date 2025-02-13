// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	securityv1alpha1constants "github.com/gardener/gardener/pkg/apis/security/v1alpha1/constants"
	"golang.org/x/oauth2/google/externalaccount"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	// TokenRetriever can be used to retrieve token that is going to be exchanged for a GCP access token.
	// This can be used instead of TokenFilePath when the token should be retrieved programmatically.
	TokenRetriever externalaccount.SubjectTokenSupplier
	// Audience is the intended audience.
	Audience string
	// UniverseDomain is the universe domain.
	UniverseDomain string
	// TokenURL is the url used for token exchange.
	TokenURL string
	// SubjectTokenType is the type of the subject token.
	// Currently only "urn:ietf:params:oauth:token-type:jwt" is supported.
	SubjectTokenType string
}

// GetCredentialsConfigFromSecretReference retrieves the credentials config from the secret with the given secret reference.
func GetCredentialsConfigFromSecretReference(ctx context.Context, c client.Client, secretRef corev1.SecretReference) (*CredentialsConfig, error) {
	secret, err := extensionscontroller.GetSecretByReference(ctx, c, &secretRef)
	if err != nil {
		return nil, err
	}

	credentialsConfig, err := getCredentialsConfigFromSecret(secret)
	if err != nil {
		return nil, err
	}

	if credentialsConfig.Type == ExternalAccountCredentialType {
		credentialsConfig.TokenRetriever = &tokenRetriever{
			c:               c,
			secretName:      secretRef.Name,
			secretNamespace: secretRef.Namespace,
		}
	}

	return credentialsConfig, nil
}

// getCredentialsConfigFromSecret retrieves the credentials config from the secret.
func getCredentialsConfigFromSecret(secret *corev1.Secret) (*CredentialsConfig, error) {
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
		Raw:              data,
		ProjectID:        credentialsConfig.ProjectID,
		Email:            credentialsConfig.Email,
		Type:             credentialsConfig.Type,
		TokenFilePath:    credentialsConfig.CredentialSource.File,
		Audience:         credentialsConfig.Audience,
		UniverseDomain:   credentialsConfig.UniverseDomain,
		TokenURL:         credentialsConfig.TokenURL,
		SubjectTokenType: credentialsConfig.SubjectTokenType,
	}, nil
}

type tokenRetriever struct {
	c               client.Client
	secretName      string
	secretNamespace string
}

var _ externalaccount.SubjectTokenSupplier = &tokenRetriever{}

func (t *tokenRetriever) SubjectToken(ctx context.Context, _ externalaccount.SupplierOptions) (string, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: t.secretNamespace,
			Name:      t.secretName,
		},
	}

	if err := t.c.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
		return "", err
	}

	if secret.Labels[securityv1alpha1constants.LabelPurpose] != securityv1alpha1constants.LabelPurposeWorkloadIdentityTokenRequestor {
		return "", errors.New("secret is not with purpose " + securityv1alpha1constants.LabelPurposeWorkloadIdentityTokenRequestor)
	}

	token, ok := secret.Data[securityv1alpha1constants.DataKeyToken]
	if !ok {
		return "", errors.New("secret does not contain a token")
	}

	return string(token), nil
}
