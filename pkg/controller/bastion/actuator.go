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

	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"

	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/internal/client"
	"github.com/gardener/gardener/extensions/pkg/controller/bastion"
	"github.com/gardener/gardener/extensions/pkg/controller/common"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	corev1 "k8s.io/api/core/v1"
)

// SSHPort is the SSH Port use in createFirewallRule
const (
	SSHPort = 22
)

type actuator struct {
	common.ClientContext

	logger logr.Logger
}

func newActuator() bastion.Actuator {
	return &actuator{
		logger: logger,
	}
}

func (a *actuator) getGCPClient(ctx context.Context, bastion *extensionsv1alpha1.Bastion) (gcpclient.Interface, error) {
	secret := &corev1.Secret{}
	key := kubernetes.Key(bastion.Namespace, v1beta1constants.SecretNameCloudProvider)

	if err := a.Client().Get(ctx, key, secret); err != nil {
		return nil, errors.Wrapf(err, "failed to find %q Secret", v1beta1constants.SecretNameCloudProvider)
	}

	gcpClient, err := gcpclient.NewFromServiceAccount(ctx, secret.Data[gcp.ServiceAccountJSONField])
	if err != nil {
		return nil, err
	}
	return gcpClient, nil
}

func getBastionInstance(ctx context.Context, gcpclient gcpclient.Interface, opt *Options) (*compute.Instance, error) {
	instance, err := gcpclient.Instances().Get(opt.ProjectID, opt.Zone, opt.BastionInstanceName).Context(ctx).Do()
	if err != nil {
		googleError := err.(*googleapi.Error)
		if googleError.Code == 404 {
			logger.Info("Instance not found,", "instance_name", opt.BastionInstanceName)
			return nil, nil
		}
		return nil, errors.Wrap(err, "failed to get Instance")
	}
	return instance, nil
}

func getFirewallRule(ctx context.Context, gcpclient gcpclient.Interface, opt *Options) (*compute.Firewall, error) {
	firewall, err := gcpclient.Firewalls().Get(opt.ProjectID, opt.FirewallName).Context(ctx).Do()
	if err != nil {
		var googleError = err.(*googleapi.Error)
		if googleError.Code == 404 {
			logger.Info("Firewall rule not found,", "firewall_rule_name", opt.FirewallName)
			return nil, nil
		}
		return nil, errors.Wrap(err, "failed to get Firewall")
	}

	return firewall, nil
}
