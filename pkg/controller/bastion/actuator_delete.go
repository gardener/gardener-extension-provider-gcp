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

package bastion

import (
	"context"
	"errors"
	"fmt"
	"time"

	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/internal/client"

	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/go-logr/logr"
)

func (a *actuator) Delete(ctx context.Context, log logr.Logger, bastion *extensionsv1alpha1.Bastion, cluster *controller.Cluster) error {
	serviceAccount, err := getServiceAccount(ctx, a, bastion)
	if err != nil {
		return fmt.Errorf("failed to get service account: %w", err)
	}

	gcpClient, err := createGCPClient(ctx, serviceAccount)
	if err != nil {
		return fmt.Errorf("failed to create GCP client: %w", err)
	}

	infrastructureStatus, subnet, err := getInfrastructureStatus(ctx, a.Client(), cluster)
	if err != nil {
		return err
	}

	opt, err := DetermineOptions(bastion, cluster, serviceAccount.ProjectID, infrastructureStatus.Networks.VPC.Name, subnet)
	if err != nil {
		return fmt.Errorf("failed to determine Options: %w", err)
	}

	if opt.Zone == "" {
		opt.Zone, err = getDefaultGCPZone(ctx, gcpClient, opt, cluster.Shoot.Spec.Region)
		if err != nil {
			return err
		}
	}

	if err := removeBastionInstance(ctx, log, gcpClient, opt); err != nil {
		return fmt.Errorf("failed to remove bastion instance: %w", err)
	}

	deleted, err := isInstanceDeleted(ctx, gcpClient, opt)
	if err != nil {
		return fmt.Errorf("failed to check for bastion instance: %w", err)
	}

	if !deleted {
		return &reconcilerutils.RequeueAfterError{
			RequeueAfter: 30 * time.Second,
			Cause:        errors.New("bastion instance is still deleting"),
		}
	}

	if err := removeDisk(ctx, log, gcpClient, opt); err != nil {
		return fmt.Errorf("failed to remove disk: %w", err)
	}

	if err := removeFirewallRules(ctx, log, gcpClient, opt); err != nil {
		return fmt.Errorf("failed to remove firewall rule: %w", err)
	}

	return nil
}

func removeFirewallRules(ctx context.Context, log logr.Logger, gcpclient gcpclient.Interface, opt *Options) error {
	firewallList := []string{FirewallIngressAllowSSHResourceName(opt.BastionInstanceName), FirewallEgressDenyAllResourceName(opt.BastionInstanceName), FirewallEgressAllowOnlyResourceName(opt.BastionInstanceName)}
	for _, firewall := range firewallList {
		if err := deleteFirewallRule(ctx, log, gcpclient, opt, firewall); err != nil {
			return err
		}
	}
	return nil
}

func removeBastionInstance(ctx context.Context, logger logr.Logger, gcpclient gcpclient.Interface, opt *Options) error {
	instance, err := getBastionInstance(ctx, gcpclient, opt)
	if err != nil {
		return err
	}

	if instance == nil {
		return nil
	}

	if _, err := gcpclient.Instances().Delete(opt.ProjectID, opt.Zone, opt.BastionInstanceName).Context(ctx).Do(); err != nil {
		return fmt.Errorf("failed to terminate bastion instance: %w", err)
	}

	logger.Info("Instance removed", "instance", opt.BastionInstanceName)
	return nil
}

func isInstanceDeleted(ctx context.Context, gcpclient gcpclient.Interface, opt *Options) (bool, error) {
	instance, err := getBastionInstance(ctx, gcpclient, opt)
	if err != nil {
		return false, err
	}

	return instance == nil, nil
}

func removeDisk(ctx context.Context, logger logr.Logger, gcpclient gcpclient.Interface, opt *Options) error {
	disk, err := getDisk(ctx, gcpclient, opt)
	if err != nil {
		return err
	}

	if disk == nil {
		return nil
	}

	if _, err := gcpclient.Disks().Delete(opt.ProjectID, opt.Zone, opt.DiskName).Context(ctx).Do(); err != nil {
		return fmt.Errorf("failed to delete disk: %w", err)
	}

	logger.Info("Disk removed", "disk", opt.DiskName)
	return nil
}
