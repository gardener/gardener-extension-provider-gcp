// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"
	"regexp"

	"google.golang.org/api/iam/v1"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

var (
	serviceAccountIDRegex           = regexp.MustCompile(`^projects/.*/serviceAccounts/.*@.*\.iam\.gserviceaccount\.com$`)
	_                     IAMClient = &iamClient{}
)

// IAMClient is the client interface for the IAM API.
type IAMClient interface {
	GetServiceAccount(ctx context.Context, name string) (*iam.ServiceAccount, error)
	CreateServiceAccount(ctx context.Context, accountID string) (*iam.ServiceAccount, error)
	DeleteServiceAccount(context.Context, string) error
}

type iamClient struct {
	service   *iam.Service
	projectID string
}

// NewIAMClient returns a new IAM client.
func NewIAMClient(ctx context.Context, credentialsConfig *gcp.CredentialsConfig) (IAMClient, error) {
	conn, err := clientOptions(ctx, credentialsConfig, []string{iam.CloudPlatformScope})
	if err != nil {
		return nil, err
	}

	service, err := iam.NewService(ctx, conn)
	if err != nil {
		return nil, err
	}

	return &iamClient{
		service:   service,
		projectID: credentialsConfig.ProjectID,
	}, nil
}

func (i *iamClient) GetServiceAccount(ctx context.Context, name string) (*iam.ServiceAccount, error) {
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

func (i *iamClient) CreateServiceAccount(ctx context.Context, accountId string) (*iam.ServiceAccount, error) {
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
