// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"net/http"

	corev1 "k8s.io/api/core/v1"
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

type factory struct {
	tokenMetadataClient *http.Client
	tokenMetadataURL    func(secretName, secretNamespace string) string
}

// New returns a new instance of Factory.
func New(tokenMetadataURL func(secretName, secretNamespace string) string, tokenMetadataClient *http.Client) Factory {
	return &factory{
		tokenMetadataURL:    tokenMetadataURL,
		tokenMetadataClient: tokenMetadataClient,
	}
}

// DNS returns a GCP cloud DNS service client.
func (f factory) DNS(ctx context.Context, c client.Client, sr corev1.SecretReference) (DNSClient, error) {
	credentialsConfig, err := gcp.GetCredentialsConfigFromSecretReference(ctx, c, sr)
	if err != nil {
		return nil, err
	}
	if f.tokenMetadataClient != nil {
		_, err := credentialsConfig.InjectURLCredentialSource(f.tokenMetadataURL(sr.Name, sr.Namespace), f.tokenMetadataClient)
		if err != nil {
			return nil, err
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
	if f.tokenMetadataClient != nil {
		_, err := credentialsConfig.InjectURLCredentialSource(f.tokenMetadataURL(sr.Name, sr.Namespace), f.tokenMetadataClient)
		if err != nil {
			return nil, err
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
	if f.tokenMetadataClient != nil {
		_, err := credentialsConfig.InjectURLCredentialSource(f.tokenMetadataURL(sr.Name, sr.Namespace), f.tokenMetadataClient)
		if err != nil {
			return nil, err
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
	if f.tokenMetadataClient != nil {
		_, err := credentialsConfig.InjectURLCredentialSource(f.tokenMetadataURL(sr.Name, sr.Namespace), f.tokenMetadataClient)
		if err != nil {
			return nil, err
		}
	}
	return NewIAMClient(ctx, credentialsConfig)
}
