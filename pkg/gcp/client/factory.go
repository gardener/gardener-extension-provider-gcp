// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"errors"

	securityv1alpha1constants "github.com/gardener/gardener/pkg/apis/security/v1alpha1/constants"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/google/externalaccount"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

var (
	_ Factory = &factory{}
)

// Factory is a factory that can produce clients for various GCP Services.
type Factory interface {
	// DNS returns a GCP cloud DNS service client.
	DNS(context.Context, client.Client, corev1.SecretReference) (DNSClient, error)
	// Storage returns a GCP (blob) storage client.
	Storage(context.Context, client.Client, corev1.SecretReference) (StorageClient, error)
	// Compute returns a GCP compute client.
	Compute(context.Context, client.Client, corev1.SecretReference) (ComputeClient, error)
	// IAM returns a GCP compute client.
	IAM(context.Context, client.Client, corev1.SecretReference) (IAMClient, error)
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

type factory struct {
}

// New returns a new instance of Factory.
func New() Factory {
	return &factory{}
}

// DNS returns a GCP cloud DNS service client.
func (f factory) DNS(ctx context.Context, c client.Client, sr corev1.SecretReference) (DNSClient, error) {
	credentialsConfig, err := gcp.GetCredentialsConfigFromSecretReference(ctx, c, sr)
	if err != nil {
		return nil, err
	}

	if credentialsConfig.Type == gcp.ExternalAccountCredentialType {
		credentialsConfig.TokenRetriever = &tokenRetriever{
			c:               c,
			secretName:      sr.Name,
			secretNamespace: sr.Namespace,
		}
	}

	return NewDNSClient(ctx, credentialsConfig)
}

// Storage reads the secret from the passed reference and returns a GCP (blob) storage client.
func (f factory) Storage(ctx context.Context, c client.Client, sr corev1.SecretReference) (StorageClient, error) {
	credentialsConfig, err := gcp.GetCredentialsConfigFromSecretReference(ctx, c, sr)
	if err != nil {
		return nil, err
	}

	if credentialsConfig.Type == gcp.ExternalAccountCredentialType {
		credentialsConfig.TokenRetriever = &tokenRetriever{
			c:               c,
			secretName:      sr.Name,
			secretNamespace: sr.Namespace,
		}
	}

	return NewStorageClient(ctx, credentialsConfig)
}

// Compute reads the secret from the passed reference and returns a GCP compute client.
func (f factory) Compute(ctx context.Context, c client.Client, sr corev1.SecretReference) (ComputeClient, error) {
	credentialsConfig, err := gcp.GetCredentialsConfigFromSecretReference(ctx, c, sr)
	if err != nil {
		return nil, err
	}

	if credentialsConfig.Type == gcp.ExternalAccountCredentialType {
		credentialsConfig.TokenRetriever = &tokenRetriever{
			c:               c,
			secretName:      sr.Name,
			secretNamespace: sr.Namespace,
		}
	}

	return NewComputeClient(ctx, credentialsConfig)
}

// IAM reads the secret from the passed reference and returns a GCP compute client.
func (f factory) IAM(ctx context.Context, c client.Client, sr corev1.SecretReference) (IAMClient, error) {
	credentialsConfig, err := gcp.GetCredentialsConfigFromSecretReference(ctx, c, sr)
	if err != nil {
		return nil, err
	}

	if credentialsConfig.Type == gcp.ExternalAccountCredentialType {
		credentialsConfig.TokenRetriever = &tokenRetriever{
			c:               c,
			secretName:      sr.Name,
			secretNamespace: sr.Namespace,
		}
	}

	return NewIAMClient(ctx, credentialsConfig)
}

func tokenSource(ctx context.Context, credentialsConfig *gcp.CredentialsConfig, scopes []string) (oauth2.TokenSource, error) {
	if credentialsConfig.TokenRetriever != nil && credentialsConfig.Type == gcp.ExternalAccountCredentialType {
		conf := externalaccount.Config{
			Audience:             credentialsConfig.Audience,
			SubjectTokenType:     credentialsConfig.SubjectTokenType,
			TokenURL:             credentialsConfig.TokenURL,
			Scopes:               scopes,
			SubjectTokenSupplier: credentialsConfig.TokenRetriever,
			UniverseDomain:       credentialsConfig.UniverseDomain,
		}

		ts, err := externalaccount.NewTokenSource(ctx, conf)
		if err != nil {
			return nil, err
		}

		return ts, nil
	}

	credentials, err := google.CredentialsFromJSONWithParams(ctx, credentialsConfig.Raw, google.CredentialsParams{
		Scopes: scopes,
	})
	if err != nil {
		return nil, err
	}

	return credentials.TokenSource, nil
}
