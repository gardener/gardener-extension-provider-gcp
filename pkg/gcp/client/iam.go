// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	iam "google.golang.org/api/iam/v1"
	"google.golang.org/api/option"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

// IAMClient is the client interface for the IAM API.
type IAMClient interface {
	GetServiceAccount(ctx context.Context, name string) (*iam.ServiceAccount, error)
}

type iamClient struct {
	service   *iam.Service
	projectID string
}

// NewIAMClientFromSecretRef creates a new compute client from the given client and secret reference.
func NewIAMClientFromSecretRef(ctx context.Context, c client.Client, secretRef corev1.SecretReference) (IAMClient, error) {
	serviceAccount, err := gcp.GetServiceAccountFromSecretReference(ctx, c, secretRef)
	if err != nil {
		return nil, err
	}

	return NewIAMClient(ctx, serviceAccount)
}

// NewIAMClient returns a new IAM client.
func NewIAMClient(ctx context.Context, serviceAccount *gcp.ServiceAccount) (IAMClient, error) {
	credentials, err := google.CredentialsFromJSON(ctx, serviceAccount.Raw, iam.CloudPlatformScope)
	if err != nil {
		return nil, err
	}

	client := oauth2.NewClient(ctx, credentials.TokenSource)
	service, err := iam.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	return &iamClient{
		service:   service,
		projectID: credentials.ProjectID,
	}, nil
}

func (i *iamClient) GetServiceAccount(ctx context.Context, name string) (*iam.ServiceAccount, error) {
	return i.service.Projects.ServiceAccounts.Get(i.serviceAccountID(name)).Context(ctx).Do()
}

func (i *iamClient) serviceAccountID(baseName string) string {
	return fmt.Sprintf("projects/%s/serviceAccounts/%s@%s.iam.gserviceaccount.com", i.projectID, baseName, i.projectID)
}
