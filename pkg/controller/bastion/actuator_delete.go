// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/go-logr/logr"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/internal/client"
)

func (a *actuator) Delete(ctx context.Context, log logr.Logger, bastion *extensionsv1alpha1.Bastion, cluster *controller.Cluster) error {
	serviceAccount, err := getServiceAccount(ctx, a.client, bastion)
	if err != nil {
		return fmt.Errorf("failed to get service account: %w", err)
	}

	gcpClient, err := createGCPClient(ctx, serviceAccount)
	if err != nil {
		return util.DetermineError(fmt.Errorf("failed to create GCP client: %w", err), helper.KnownCodes)
	}

	infrastructureStatus, subnet, err := getInfrastructureStatus(ctx, a.client, cluster)
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
			return util.DetermineError(err, helper.KnownCodes)
		}
	}

	if err := removeBastionInstance(ctx, log, gcpClient, opt); err != nil {
		return util.DetermineError(fmt.Errorf("failed to remove bastion instance: %w", err), helper.KnownCodes)
	}

	deleted, err := isInstanceDeleted(ctx, gcpClient, opt)
	if err != nil {
		return util.DetermineError(fmt.Errorf("failed to check for bastion instance: %w", err), helper.KnownCodes)
	}

	if !deleted {
		return &reconcilerutils.RequeueAfterError{
			RequeueAfter: 30 * time.Second,
			Cause:        errors.New("bastion instance is still deleting"),
		}
	}

	if err := removeDisk(ctx, log, gcpClient, opt); err != nil {
		return util.DetermineError(fmt.Errorf("failed to remove disk: %w", err), helper.KnownCodes)
	}

	if err := removeFirewallRules(ctx, log, gcpClient, opt); err != nil {
		return util.DetermineError(fmt.Errorf("failed to remove firewall rule: %w", err), helper.KnownCodes)
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

func (a *actuator) ForceDelete(_ context.Context, _ logr.Logger, _ *extensionsv1alpha1.Bastion, _ *controller.Cluster) error {
	return nil
}
