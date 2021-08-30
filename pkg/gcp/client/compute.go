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

	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ComputeClient is an interface which must be implemented by GCP compute clients.
type ComputeClient interface {
	// GetExternalAddresses returns a list of all external IP addresses mapped to whether they are available for use or not.
	GetExternalAddresses(ctx context.Context, region string) (map[string]bool, error)
}

type computeClient struct {
	service   *compute.Service
	projectID string
}

func newComputeClient(ctx context.Context, serviceAccount *gcp.ServiceAccount) (ComputeClient, error) {
	credentials, err := google.CredentialsFromJSON(ctx, serviceAccount.Raw, compute.ComputeReadonlyScope)
	if err != nil {
		return nil, err
	}
	client := oauth2.NewClient(ctx, credentials.TokenSource)
	service, err := compute.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	return &computeClient{
		service:   service,
		projectID: credentials.ProjectID,
	}, nil
}

// NewComputeClientFromSecretRef creates a new compute client from the given client and secret reference.
func NewComputeClientFromSecretRef(ctx context.Context, c client.Client, secretRef corev1.SecretReference) (ComputeClient, error) {
	serviceAccount, err := gcp.GetServiceAccount(ctx, c, secretRef)
	if err != nil {
		return nil, err
	}

	return newComputeClient(ctx, serviceAccount)
}

// GetExternalAddresses returns a list of all external IP addresses mapped to whether they are available for use or not.
func (s *computeClient) GetExternalAddresses(ctx context.Context, region string) (map[string]bool, error) {
	addresses := make(map[string]bool)
	if err := s.service.Addresses.List(s.projectID, region).Pages(ctx, func(resp *compute.AddressList) error {
		for _, address := range resp.Items {
			if address.AddressType == "EXTERNAL" {
				addresses[address.Name] = address.Status == "RESERVED"
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return addresses, nil
}
