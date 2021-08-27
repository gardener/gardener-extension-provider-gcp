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

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Factory is a factory that can produce clients for various GCP Services.
type Factory interface {
	NewDNSClient(context.Context, client.Client, corev1.SecretReference) (DNSClient, error)
	NewStorageClient(context.Context, client.Client, corev1.SecretReference) (StorageClient, error)
	NewComputeClient(context.Context, client.Client, corev1.SecretReference) (ComputeClient, error)
}

type factory struct{}

// NewFactory returns a new factory to produce clients for various GCP services.
func NewFactory() Factory {
	return factory{}
}

// NewDNSClient reads the secret from the passed reference and returns a GCP cloud DNS service client.
func (f factory) NewDNSClient(ctx context.Context, client client.Client, secretRef corev1.SecretReference) (DNSClient, error) {
	return NewDNSClientFromSecretRef(ctx, client, secretRef)
}

// NewStorageClient reads the secret from the passed reference and returns a GCP (blob) storage client.
func (f factory) NewStorageClient(ctx context.Context, client client.Client, secretRef corev1.SecretReference) (StorageClient, error) {
	return NewStorageClientFromSecretRef(ctx, client, secretRef)
}

// NewComputeClient reads the secret from the passed reference and returns a GCP compute client.
func (f factory) NewComputeClient(ctx context.Context, client client.Client, secretRef corev1.SecretReference) (ComputeClient, error) {
	return NewComputeClientFromSecretRef(ctx, client, secretRef)
}
