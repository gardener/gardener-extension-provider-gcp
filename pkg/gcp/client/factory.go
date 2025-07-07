// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"

	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/google/externalaccount"
	"google.golang.org/api/option"
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

	return NewDNSClient(ctx, credentialsConfig)
}

// Storage reads the secret from the passed reference and returns a GCP (blob) storage client.
func (f factory) Storage(ctx context.Context, c client.Client, sr corev1.SecretReference) (StorageClient, error) {
	credentialsConfig, err := gcp.GetCredentialsConfigFromSecretReference(ctx, c, sr)
	if err != nil {
		return nil, err
	}

	return NewStorageClient(ctx, credentialsConfig)
}

// Compute reads the secret from the passed reference and returns a GCP compute client.
func (f factory) Compute(ctx context.Context, c client.Client, sr corev1.SecretReference) (ComputeClient, error) {
	credentialsConfig, err := gcp.GetCredentialsConfigFromSecretReference(ctx, c, sr)
	if err != nil {
		return nil, err
	}

	return NewComputeClient(ctx, credentialsConfig)
}

// IAM reads the secret from the passed reference and returns a GCP compute client.
func (f factory) IAM(ctx context.Context, c client.Client, sr corev1.SecretReference) (IAMClient, error) {
	credentialsConfig, err := gcp.GetCredentialsConfigFromSecretReference(ctx, c, sr)
	if err != nil {
		return nil, err
	}

	return NewIAMClient(ctx, credentialsConfig)
}

func clientOptions(ctx context.Context, credentialsConfig *gcp.CredentialsConfig, scopes []string) ([]option.ClientOption, error) {
	// Note: Incompatible with "WithHTTPClient"
	UAOption := option.WithUserAgent("Gardener Extension for GCP provider")

	switch {
	case credentialsConfig.TokenRetriever != nil && credentialsConfig.Type == gcp.ExternalAccountCredentialType:
		conf := externalaccount.Config{
			Audience:                       credentialsConfig.Audience,
			SubjectTokenType:               credentialsConfig.SubjectTokenType,
			TokenURL:                       credentialsConfig.TokenURL,
			Scopes:                         scopes,
			SubjectTokenSupplier:           credentialsConfig.TokenRetriever,
			UniverseDomain:                 credentialsConfig.UniverseDomain,
			ServiceAccountImpersonationURL: credentialsConfig.ServiceAccountImpersonationURL,
		}

		ts, err := externalaccount.NewTokenSource(ctx, conf)
		if err != nil {
			return nil, err
		}
		return []option.ClientOption{option.WithTokenSource(ts), UAOption}, nil

	case credentialsConfig.Type == gcp.ServiceAccountCredentialType:
		jwt, err := google.JWTConfigFromJSON(credentialsConfig.Raw, scopes...)
		if err != nil {
			return nil, err
		}
		return []option.ClientOption{option.WithTokenSource(jwt.TokenSource(ctx)), UAOption}, nil

	default:
		return nil, fmt.Errorf("unknow credential type: %s", credentialsConfig.Type)
	}
}
