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

func newDNSService(ctx context.Context, serviceAccount *gcp.ServiceAccount) (DNS, error) {
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

func newDNSServiceFromSecretRef(ctx context.Context, c client.Client, secretRef corev1.SecretReference) (DNS, error) {
	serviceAccount, err := gcp.GetServiceAccount(ctx, c, secretRef)
	if err != nil {
		return nil, err
	}

	return newDNSService(ctx, serviceAccount)
}

// GetHostedZones returns a map of all zone DNS names mapped to their user assigned resource names.
func (s *dnsClient) GetHostedZones(ctx context.Context) (map[string]string, error) {
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

// CreateOrUpdateRecordSet creates or updates the ResourceRecordSet with the given name, record type, records, and ttl
// in the zone with the given zone ID.
func (s *dnsClient) CreateOrUpdateRecordSet(ctx context.Context, zoneId, name, recordType string, records []string, ttl int64) error {
	name = ensureTrailingDot(name)
	rrs, err := s.getResourceRecordSet(ctx, zoneId, name, recordType)
	if err != nil {
		return err
	}
	records = formatRecords(recordType, records)
	change := &googledns.Change{}
	if rrs != nil {
		if reflect.DeepEqual(rrs.Rrdatas, records) && rrs.Ttl == ttl {
			return nil
		}
		change.Deletions = append(change.Deletions, rrs)
	}
	change.Additions = append(change.Additions, &googledns.ResourceRecordSet{Name: name, Type: recordType, Rrdatas: records, Ttl: ttl})
	_, err = s.service.Changes.Create(s.projectID, zoneId, change).Do()
	return err
}

// DeleteRecordSet deletes the recordset with the given name and record type in the zone with the given zone ID.
func (s *dnsClient) DeleteRecordSet(ctx context.Context, zoneId, name, recordType string) error {
	name = ensureTrailingDot(name)
	rrs, err := s.getResourceRecordSet(ctx, zoneId, name, recordType)
	if err != nil {
		return err
	}
	if rrs == nil {
		return nil
	}
	change := &googledns.Change{
		Deletions: []*googledns.ResourceRecordSet{{Name: rrs.Name, Type: rrs.Type, Rrdatas: rrs.Rrdatas, Ttl: rrs.Ttl}},
	}
	_, err = s.service.Changes.Create(s.projectID, zoneId, change).Do()
	return err
}

func (s *dnsClient) getResourceRecordSet(ctx context.Context, zoneId, name, recordType string) (*googledns.ResourceRecordSet, error) {
	resp, err := s.service.ResourceRecordSets.List(s.projectID, zoneId).Name(name).Type(recordType).Do()
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

func formatRecords(recordType string, values []string) []string {
	records := make([]string, len(values))
	for i, val := range values {
		if recordType == "CNAME" {
			records[i] = ensureTrailingDot(val)
		} else {
			records[i] = val
		}
	}
	return records
}

func ensureTrailingDot(host string) string {
	if strings.HasSuffix(host, ".") {
		return host
	}
	return host + "."
}
