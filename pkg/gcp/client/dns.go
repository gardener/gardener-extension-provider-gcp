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
	"reflect"
	"strings"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	googledns "google.golang.org/api/dns/v1"
	"google.golang.org/api/option"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DNSClient is an interface which must be implemented by GCP DNS clients.
type DNSClient interface {
	GetManagedZones(ctx context.Context) (map[string]string, error)
	CreateOrUpdateRecordSet(ctx context.Context, managedZone, name, recordType string, rrdatas []string, ttl int64) error
	DeleteRecordSet(ctx context.Context, managedZone, name, recordType string) error
}

type dnsClient struct {
	service   *googledns.Service
	projectID string
}

func newDNSService(ctx context.Context, serviceAccount *gcp.ServiceAccount) (DNSClient, error) {
	credentials, err := google.CredentialsFromJSON(ctx, serviceAccount.Raw, googledns.NdevClouddnsReadwriteScope)
	if err != nil {
		return nil, err
	}
	client := oauth2.NewClient(ctx, credentials.TokenSource)
	service, err := googledns.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	return &dnsClient{
		service:   service,
		projectID: credentials.ProjectID,
	}, nil
}

// NewDNSClientFromSecretRef creates a new DNS client from the given client and secret reference.
func NewDNSClientFromSecretRef(ctx context.Context, c client.Client, secretRef corev1.SecretReference) (DNSClient, error) {
	serviceAccount, err := gcp.GetServiceAccount(ctx, c, secretRef)
	if err != nil {
		return nil, err
	}

	return newDNSService(ctx, serviceAccount)
}

// GetManagedZones returns a map of all managed zone DNS names mapped to their user assigned resource names.
func (s *dnsClient) GetManagedZones(ctx context.Context) (map[string]string, error) {
	zones := make(map[string]string)
	f := func(resp *googledns.ManagedZonesListResponse) error {
		for _, zone := range resp.ManagedZones {
			zones[normalizeZoneName(zone.DnsName)] = zone.Name
		}
		return nil
	}

	if err := s.service.ManagedZones.List(s.projectID).Pages(ctx, f); err != nil {
		return nil, err
	}
	return zones, nil
}

// CreateOrUpdateRecordSet creates or updates the resource recordset with the given name, record type, rrdatas, and ttl
// in the managed zone with the given name.
func (s *dnsClient) CreateOrUpdateRecordSet(ctx context.Context, managedZone, name, recordType string, rrdatas []string, ttl int64) error {
	name = ensureTrailingDot(name)
	rrs, err := s.getResourceRecordSet(ctx, managedZone, name, recordType)
	if err != nil {
		return err
	}
	rrdatas = formatRrdatas(recordType, rrdatas)
	change := &googledns.Change{}
	if rrs != nil {
		if reflect.DeepEqual(rrs.Rrdatas, rrdatas) && rrs.Ttl == ttl {
			return nil
		}
		change.Deletions = append(change.Deletions, rrs)
	}
	change.Additions = append(change.Additions, &googledns.ResourceRecordSet{Name: name, Type: recordType, Rrdatas: rrdatas, Ttl: ttl})
	_, err = s.service.Changes.Create(s.projectID, managedZone, change).Context(ctx).Do()
	return err
}

// DeleteRecordSet deletes the resource recordset with the given name and record type in the managed zone with the given name.
func (s *dnsClient) DeleteRecordSet(ctx context.Context, managedZone, name, recordType string) error {
	name = ensureTrailingDot(name)
	rrs, err := s.getResourceRecordSet(ctx, managedZone, name, recordType)
	if err != nil {
		return err
	}
	if rrs == nil {
		return nil
	}
	change := &googledns.Change{
		Deletions: []*googledns.ResourceRecordSet{rrs},
	}
	_, err = s.service.Changes.Create(s.projectID, managedZone, change).Context(ctx).Do()
	return err
}

func (s *dnsClient) getResourceRecordSet(ctx context.Context, managedZone, name, recordType string) (*googledns.ResourceRecordSet, error) {
	resp, err := s.service.ResourceRecordSets.List(s.projectID, managedZone).Context(ctx).Name(name).Type(recordType).Do()
	if err != nil {
		return nil, err
	}
	if len(resp.Rrsets) > 0 {
		return resp.Rrsets[0], nil
	}
	return nil, nil
}

func normalizeZoneName(zoneName string) string {
	if strings.HasPrefix(zoneName, "\\052.") {
		zoneName = "*" + zoneName[4:]
	}
	if strings.HasSuffix(zoneName, ".") {
		return zoneName[:len(zoneName)-1]
	}
	return zoneName
}

func formatRrdatas(recordType string, values []string) []string {
	rrdatas := make([]string, len(values))
	for i, value := range values {
		if recordType == "CNAME" {
			rrdatas[i] = ensureTrailingDot(value)
		} else {
			rrdatas[i] = value
		}
	}
	return rrdatas
}

func ensureTrailingDot(host string) string {
	if strings.HasSuffix(host, ".") {
		return host
	}
	return host + "."
}
