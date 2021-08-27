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

package infrastructure

import (
	"context"
	"fmt"

	api "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client"

	"github.com/gardener/gardener/extensions/pkg/controller/common"
	"github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// configValidator implements ConfigValidator for GCP infrastructure resources.
type configValidator struct {
	common.ClientContext
	gcpClientFactory gcpclient.Factory
	logger           logr.Logger
}

// NewConfigValidator creates a new ConfigValidator.
func NewConfigValidator(gcpClientFactory gcpclient.Factory, logger logr.Logger) infrastructure.ConfigValidator {
	return &configValidator{
		gcpClientFactory: gcpClientFactory,
		logger:           logger.WithName("gcp-infrastructure-config-validator"),
	}
}

// Validate validates the provider config of the given infrastructure resource with the cloud provider.
func (c *configValidator) Validate(ctx context.Context, infra *extensionsv1alpha1.Infrastructure) field.ErrorList {
	allErrs := field.ErrorList{}

	logger := c.logger.WithValues("infrastructure", client.ObjectKeyFromObject(infra))

	// Get provider config from the infrastructure resource
	config, err := helper.InfrastructureConfigFromInfrastructure(infra)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(nil, err))
		return allErrs
	}

	// Create GCP compute client
	computeClient, err := c.gcpClientFactory.NewComputeClient(ctx, c.Client(), infra.Spec.SecretRef)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(nil, err))
		return allErrs
	}

	// Validate infrastructure config
	if config.Networks.CloudNAT != nil {
		logger.Info("Validating infrastructure networks.cloudNAT configuration")
		allErrs = append(allErrs, c.validateCloudNAT(ctx, computeClient, infra.Spec.Region, config.Networks.CloudNAT, field.NewPath("networks", "cloudNAT"))...)
	}

	return allErrs
}

func (c *configValidator) validateCloudNAT(ctx context.Context, computeClient gcpclient.ComputeClient, region string, cloudNAT *api.CloudNAT, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(cloudNAT.NatIPNames) == 0 {
		return allErrs
	}

	// Get external IP addresses mapped to whether they are available or not
	externalAddresses, err := computeClient.GetExternalAddresses(ctx, region)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, fmt.Errorf("could not get external IP addresses: %w", err)))
		return allErrs
	}

	// Check whether each specified NAT IP name exists and is available
	for i, natIP := range cloudNAT.NatIPNames {
		natIPNamePath := fldPath.Child("natIPNames").Index(i).Child("name")
		if available, ok := externalAddresses[natIP.Name]; !ok {
			allErrs = append(allErrs, field.NotFound(natIPNamePath, natIP.Name))
		} else if !available {
			allErrs = append(allErrs, field.Invalid(natIPNamePath, natIP.Name, "external IP address is already in use"))
		}
	}

	return allErrs
}
