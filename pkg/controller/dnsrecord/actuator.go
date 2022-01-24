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

package dnsrecord

import (
	"context"
	"fmt"
	"time"

	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/common"
	"github.com/gardener/gardener/extensions/pkg/controller/dnsrecord"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// requeueAfterOnProviderError is a value for RequeueAfter to be returned on provider errors
	// in order to prevent quick retries that could quickly exhaust the account rate limits in case of e.g.
	// configuration issues.
	requeueAfterOnProviderError = 30 * time.Second
)

type actuator struct {
	common.ClientContext
	gcpClientFactory gcpclient.Factory
	logger           logr.Logger
}

// NewActuator creates a new dnsrecord.Actuator.
func NewActuator(gcpClientFactory gcpclient.Factory, logger logr.Logger) dnsrecord.Actuator {
	return &actuator{
		gcpClientFactory: gcpClientFactory,
		logger:           logger.WithName("gcp-dnsrecord-actuator"),
	}
}

// Reconcile reconciles the DNSRecord.
func (a *actuator) Reconcile(ctx context.Context, dns *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) error {
	// Create GCP DNS client
	dnsClient, err := a.gcpClientFactory.NewDNSClient(ctx, a.Client(), dns.Spec.SecretRef)
	if err != nil {
		return err
	}

	// Determine DNS managed zone
	managedZone, err := a.getManagedZone(ctx, dns, dnsClient)
	if err != nil {
		return err
	}

	// Create or update DNS recordset
	ttl := extensionsv1alpha1helper.GetDNSRecordTTL(dns.Spec.TTL)
	a.logger.Info("Creating or updating DNS recordset", "managedZone", managedZone, "name", dns.Spec.Name, "type", dns.Spec.RecordType, "rrdatas", dns.Spec.Values, "dnsrecord", kutil.ObjectName(dns))
	if err := dnsClient.CreateOrUpdateRecordSet(ctx, managedZone, dns.Spec.Name, string(dns.Spec.RecordType), dns.Spec.Values, ttl); err != nil {
		return &reconcilerutils.RequeueAfterError{
			Cause:        fmt.Errorf("could not create or update DNS recordset in managed zone %s with name %s, type %s, and rrdatas %v: %+v", managedZone, dns.Spec.Name, dns.Spec.RecordType, dns.Spec.Values, err),
			RequeueAfter: requeueAfterOnProviderError,
		}
	}

	// Delete meta DNS recordset if exists
	if dns.Status.LastOperation == nil || dns.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeCreate {
		name, recordType := dnsrecord.GetMetaRecordName(dns.Spec.Name), "TXT"
		a.logger.Info("Deleting meta DNS recordset", "managedZone", managedZone, "name", name, "type", recordType, "dnsrecord", kutil.ObjectName(dns))
		if err := dnsClient.DeleteRecordSet(ctx, managedZone, name, recordType); err != nil {
			return &reconcilerutils.RequeueAfterError{
				Cause:        fmt.Errorf("could not delete meta DNS recordset in managed zone %s with name %s and type %s: %+v", managedZone, name, recordType, err),
				RequeueAfter: requeueAfterOnProviderError,
			}
		}
	}

	// Update resource status
	patch := client.MergeFrom(dns.DeepCopy())
	dns.Status.Zone = &managedZone
	return a.Client().Status().Patch(ctx, dns, patch)
}

// Delete deletes the DNSRecord.
func (a *actuator) Delete(ctx context.Context, dns *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) error {
	// Create GCP DNS client
	dnsClient, err := a.gcpClientFactory.NewDNSClient(ctx, a.Client(), dns.Spec.SecretRef)
	if err != nil {
		return err
	}

	// Determine DNS managed zone
	managedZone, err := a.getManagedZone(ctx, dns, dnsClient)
	if err != nil {
		return err
	}

	// Delete DNS recordset
	a.logger.Info("Deleting DNS recordset", "managedZone", managedZone, "name", dns.Spec.Name, "type", dns.Spec.RecordType, "dnsrecord", kutil.ObjectName(dns))
	if err := dnsClient.DeleteRecordSet(ctx, managedZone, dns.Spec.Name, string(dns.Spec.RecordType)); err != nil {
		return &reconcilerutils.RequeueAfterError{
			Cause:        fmt.Errorf("could not delete DNS recordset in managed zone %s with name %s and type %s: %+v", managedZone, dns.Spec.Name, dns.Spec.RecordType, err),
			RequeueAfter: requeueAfterOnProviderError,
		}
	}
	return nil
}

// Restore restores the DNSRecord.
func (a *actuator) Restore(ctx context.Context, dns *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) error {
	return a.Reconcile(ctx, dns, cluster)
}

// Migrate migrates the DNSRecord.
func (a *actuator) Migrate(context.Context, *extensionsv1alpha1.DNSRecord, *extensionscontroller.Cluster) error {
	return nil
}

func (a *actuator) getManagedZone(ctx context.Context, dns *extensionsv1alpha1.DNSRecord, dnsClient gcpclient.DNSClient) (string, error) {
	switch {
	case dns.Spec.Zone != nil && *dns.Spec.Zone != "":
		return *dns.Spec.Zone, nil
	case dns.Status.Zone != nil && *dns.Status.Zone != "":
		return *dns.Status.Zone, nil
	default:
		// The zone is not specified in the resource status or spec. Try to determine the zone by
		// getting all managed zones of the account and searching for the longest zone name that is a suffix of dns.spec.Name
		zones, err := dnsClient.GetManagedZones(ctx)
		if err != nil {
			return "", &reconcilerutils.RequeueAfterError{
				Cause:        fmt.Errorf("could not get DNS managed zones: %+v", err),
				RequeueAfter: requeueAfterOnProviderError,
			}
		}
		a.logger.Info("Got DNS managed zones", "zones", zones, "dnsrecord", kutil.ObjectName(dns))
		zone := dnsrecord.FindZoneForName(zones, dns.Spec.Name)
		if zone == "" {
			return "", fmt.Errorf("could not find DNS managed zone for name %s", dns.Spec.Name)
		}
		return zone, nil
	}
}
