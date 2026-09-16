// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"fmt"
	"strings"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client"
)

// GCP subnet stack types, see https://cloud.google.com/vpc/docs/subnets#stack-type.
const (
	stackTypeIPv4Only = "IPV4_ONLY"
	stackTypeIPv4IPv6 = "IPV4_IPV6"
	stackTypeIPv6Only = "IPV6_ONLY"
)

// configValidator implements ConfigValidator for GCP infrastructure resources.
type configValidator struct {
	reader           client.Reader
	logger           logr.Logger
	gcpClientFactory gcpclient.Factory
}

// NewConfigValidator creates a new ConfigValidator.
func NewConfigValidator(mgr manager.Manager, logger logr.Logger, gcpClientFactory gcpclient.Factory) infrastructure.ConfigValidator {
	return &configValidator{
		reader:           mgr.GetClient(),
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
	computeClient, err := c.gcpClientFactory.Compute(ctx, c.reader, infra.Spec.SecretRef)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(nil, err))
		return allErrs
	}

	// Validate infrastructure config
	logger.Info("Validating infrastructure networks configuration")
	allErrs = append(allErrs, c.validateNetworks(ctx, computeClient, infra.Namespace, infra.Spec.Region, config.Networks, field.NewPath("networks"))...)

	// In BYO subnet mode, validate the user-managed subnets against the cluster networking.
	if config.Networks.SubnetWorkers != nil {
		networking, err := c.clusterNetworking(ctx, infra.Namespace)
		if err != nil {
			allErrs = append(allErrs, field.InternalError(field.NewPath("networks"), err))
			return allErrs
		}
		if networking != nil {
			allErrs = append(allErrs, c.validateUserManagedWorkerSubnet(ctx, computeClient, infra.Spec.Region, config.Networks.SubnetWorkers, networking, field.NewPath("networks", "subnetWorkers"))...)
			if config.Networks.SubnetServices != nil {
				allErrs = append(allErrs, c.validateUserManagedServicesSubnet(ctx, computeClient, infra.Spec.Region, config.Networks.SubnetServices, networking, field.NewPath("networks", "subnetServices"))...)
			}
		}
	}

	return allErrs
}

// clusterNetworking reads the shoot networking configuration from the Cluster resource in the given namespace.
func (c *configValidator) clusterNetworking(ctx context.Context, namespace string) (*gardencorev1beta1.Networking, error) {
	cluster, err := extensionscontroller.GetCluster(ctx, c.reader, namespace)
	if err != nil {
		return nil, fmt.Errorf("could not get cluster for namespace %q: %w", namespace, err)
	}
	return cluster.Shoot.Spec.Networking, nil
}

// validateUserManagedWorkerSubnet looks up the user-provided worker subnet and validates that its CIDR
// is a subset of the cluster's node network, does not overlap with the pod and service networks, and that
// its stack type is compatible with the cluster's IP families.
func (c *configValidator) validateUserManagedWorkerSubnet(ctx context.Context, computeClient gcpclient.ComputeClient, region string, subnetRef *apisgcp.SubnetReference, networking *gardencorev1beta1.Networking, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	subnet, err := computeClient.GetSubnet(ctx, region, subnetRef.Name)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, fmt.Errorf("could not get user-managed worker subnet %q: %w", subnetRef.Name, err)))
		return allErrs
	}
	if subnet == nil {
		allErrs = append(allErrs, field.NotFound(fldPath.Child("name"), subnetRef.Name))
		return allErrs
	}

	allErrs = append(allErrs, validateWorkerSubnetCIDRRelationships(subnet.IpCidrRange, networking, fldPath)...)
	allErrs = append(allErrs, validateSubnetStackType(subnet.StackType, networking.IPFamilies, fldPath)...)
	return allErrs
}

// validateUserManagedServicesSubnet looks up the user-provided services subnet (dual-stack BYO mode only)
// and validates that its stack type is compatible with the cluster's IP families.
func (c *configValidator) validateUserManagedServicesSubnet(ctx context.Context, computeClient gcpclient.ComputeClient, region string, subnetRef *apisgcp.SubnetReference, networking *gardencorev1beta1.Networking, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	subnet, err := computeClient.GetSubnet(ctx, region, subnetRef.Name)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, fmt.Errorf("could not get user-managed services subnet %q: %w", subnetRef.Name, err)))
		return allErrs
	}
	if subnet == nil {
		allErrs = append(allErrs, field.NotFound(fldPath.Child("name"), subnetRef.Name))
		return allErrs
	}

	allErrs = append(allErrs, validateSubnetStackType(subnet.StackType, networking.IPFamilies, fldPath)...)
	return allErrs
}

// validateSubnetStackType validates that a GCP subnet's stack type provides the IP families the cluster requires.
// An unset stack type is treated as IPV4_ONLY, matching the GCP default. IPv6-only clusters are not validated here.
func validateSubnetStackType(stackType string, ipFamilies []gardencorev1beta1.IPFamily, fldPath *field.Path) field.ErrorList {
	if stackType == "" {
		stackType = stackTypeIPv4Only
	}

	stackTypePath := fldPath.Child("stackType")
	if !gardencorev1beta1.IsIPv4SingleStack(ipFamilies) {
		// Dual-stack clusters require the subnet to support both IPv4 and IPv6.
		if stackType != stackTypeIPv4IPv6 {
			return field.ErrorList{field.Invalid(stackTypePath, stackType, fmt.Sprintf("subnet must have stack type %q for a dual-stack cluster", stackTypeIPv4IPv6))}
		}
		return nil
	}

	// IPv4 single-stack clusters require the subnet to provide IPv4 addresses.
	if stackType == stackTypeIPv6Only {
		return field.ErrorList{field.Invalid(stackTypePath, stackType, "subnet must provide IPv4 addresses for an IPv4 cluster")}
	}
	return nil
}

// validateWorkerSubnetCIDRRelationships validates that the worker subnet CIDR is a subset of the node
// network and does not overlap with the pod and service networks.
func validateWorkerSubnetCIDRRelationships(workerSubnetCIDR string, networking *gardencorev1beta1.Networking, subnetPath *field.Path) field.ErrorList {
	networkingPath := field.NewPath("networking")

	workerCIDR := cidrvalidation.NewCIDR(workerSubnetCIDR, subnetPath)
	if errs := cidrvalidation.ValidateCIDRParse(workerCIDR); len(errs) > 0 {
		return errs
	}

	var allErrs field.ErrorList
	if networking.Nodes != nil {
		nodes := cidrvalidation.NewCIDR(*networking.Nodes, networkingPath.Child("nodes"))
		allErrs = append(allErrs, workerCIDR.ValidateSubset(nodes)...)
	}
	if networking.Pods != nil {
		pods := cidrvalidation.NewCIDR(*networking.Pods, networkingPath.Child("pods"))
		allErrs = append(allErrs, workerCIDR.ValidateNotOverlap(pods)...)
	}
	if networking.Services != nil {
		services := cidrvalidation.NewCIDR(*networking.Services, networkingPath.Child("services"))
		allErrs = append(allErrs, workerCIDR.ValidateNotOverlap(services)...)
	}
	return allErrs
}

func (c *configValidator) validateNetworks(ctx context.Context, computeClient gcpclient.ComputeClient, clusterName, region string, networks apisgcp.NetworkConfig, fldPath *field.Path) field.ErrorList {
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
