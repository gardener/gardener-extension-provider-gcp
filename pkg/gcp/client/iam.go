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
	"regexp"

	"golang.org/x/oauth2/google"
	iam "google.golang.org/api/iam/v1"
	"google.golang.org/api/option"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

var (
	serviceAccountIDRegex           = regexp.MustCompile(`^projects/.*/serviceAccounts/.*@.*\.iam\.gserviceaccount\.com$`)
	_                     IAMClient = &iamClient{}
)

// IAMClient is the client interface for the IAM API.
type IAMClient interface {
	GetServiceAccount(ctx context.Context, name string) (*ServiceAccount, error)
	CreateServiceAccount(ctx context.Context, accountID string) (*ServiceAccount, error)
	DeleteServiceAccount(context.Context, string) error
}

type iamClient struct {
	service   *iam.Service
	projectID string
}

// NewIAMClient returns a new IAM client.
func NewIAMClient(ctx context.Context, serviceAccount *gcp.ServiceAccount) (IAMClient, error) {
	credentials, err := google.CredentialsFromJSON(ctx, serviceAccount.Raw, iam.CloudPlatformScope)
	if err != nil {
		return nil, err
	}

	service, err := iam.NewService(ctx, option.WithCredentials(credentials))
	if err != nil {
		return nil, err
	}

	return &iamClient{
		service:   service,
		projectID: credentials.ProjectID,
	}, nil
}

func (i *iamClient) GetServiceAccount(ctx context.Context, name string) (*ServiceAccount, error) {
	accountID := name
	if !serviceAccountIDRegex.MatchString(accountID) {
		accountID = i.serviceAccountID(name)
	}

	sa, err := i.service.Projects.ServiceAccounts.Get(accountID).Context(ctx).Do()
	if err != nil {
		return nil, IgnoreNotFoundError(err)
	}
	return sa, nil
}

func (i *iamClient) serviceAccountID(baseName string) string {
	return fmt.Sprintf("projects/%s/serviceAccounts/%s@%s.iam.gserviceaccount.com", i.projectID, baseName, i.projectID)
}

func (i *iamClient) CreateServiceAccount(ctx context.Context, accountId string) (*ServiceAccount, error) {
	name := "projects/" + i.projectID
	return i.service.Projects.ServiceAccounts.Create(name, &iam.CreateServiceAccountRequest{
		AccountId: accountId,
		ServiceAccount: &iam.ServiceAccount{
			DisplayName: accountId,
		},
	}).Context(ctx).Do()
}

func (i *iamClient) DeleteServiceAccount(ctx context.Context, name string) error {
	accountID := name
	if !serviceAccountIDRegex.MatchString(accountID) {
		accountID = i.serviceAccountID(name)
	}

	_, err := i.service.Projects.ServiceAccounts.Delete(accountID).Context(ctx).Do()
	return IgnoreNotFoundError(err)
}
