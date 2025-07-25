// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"fmt"
	"strings"

	"github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	api "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client"
)

// configValidator implements ConfigValidator for GCP infrastructure resources.
type configValidator struct {
	client           client.Client
	logger           logr.Logger
	gcpClientFactory gcpclient.Factory
}

// NewConfigValidator creates a new ConfigValidator.
func NewConfigValidator(mgr manager.Manager, logger logr.Logger, gcpClientFactory gcpclient.Factory) infrastructure.ConfigValidator {
	return &configValidator{
		client:           mgr.GetClient(),
		logger:           logger.WithName("gcp-infrastructure-config-validator"),
		gcpClientFactory: gcpClientFactory,
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
	computeClient, err := c.gcpClientFactory.Compute(ctx, c.client, infra.Spec.SecretRef)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(nil, err))
		return allErrs
	}

	// Validate infrastructure config
	logger.Info("Validating infrastructure networks configuration")
	allErrs = append(allErrs, c.validateNetworks(ctx, computeClient, infra.Namespace, infra.Spec.Region, config.Networks, field.NewPath("networks"))...)

	return allErrs
}

func (c *configValidator) validateNetworks(ctx context.Context, computeClient gcpclient.ComputeClient, clusterName, region string, networks api.NetworkConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if networks.CloudNAT == nil || len(networks.CloudNAT.NatIPNames) == 0 {
		return allErrs
	}

	// Get external IP addresses mapped to the names of their users
	externalAddresses, err := computeClient.GetExternalAddresses(ctx, region)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, fmt.Errorf("could not get external IP addresses: %w", err)))
		return allErrs
	}

	cloudRouterName := clusterName + "-cloud-router"
	if networks.VPC != nil && networks.VPC.CloudRouter != nil && len(networks.VPC.CloudRouter.Name) > 0 {
		cloudRouterName = networks.VPC.CloudRouter.Name
	}

	// Check whether each specified NAT IP name exists and is available
	for i, natIP := range networks.CloudNAT.NatIPNames {
		natIPNamePath := fldPath.Child("cloudNAT", "natIPNames").Index(i).Child("name")
		if userNames, ok := externalAddresses[natIP.Name]; !ok {
			allErrs = append(allErrs, field.NotFound(natIPNamePath, natIP.Name))
		} else if len(userNames) > 1 || len(userNames) == 1 && userNames[0] != cloudRouterName {
			allErrs = append(allErrs, field.Invalid(natIPNamePath, natIP.Name,
				fmt.Sprintf("external IP address is already in use by %s", strings.Join(userNames, ","))))
		}
	}

	return allErrs
}
