// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	googledns "google.golang.org/api/dns/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Factory is a factory that can produce clients for various GCP Services.
type Factory interface {
	NewDNSClient(context.Context, client.Client, corev1.SecretReference) (DNS, error)
	NewStorageClient(context.Context, client.Client, corev1.SecretReference) (StorageClient, error)
}

type factory struct{}

// DNS describes the operations of a client interacting with GCP's Cloud DNS service.
type DNS interface {
	GetHostedZones(ctx context.Context) (map[string]string, error)
	CreateOrUpdateRecordSet(ctx context.Context, zoneID, name, recordType string, values []string, ttl int64) error
	DeleteRecordSet(ctx context.Context, zoneID, name, recordType string) error
}

type dnsClient struct {
	service   *googledns.Service
	projectID string
}
