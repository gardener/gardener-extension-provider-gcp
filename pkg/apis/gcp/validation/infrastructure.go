// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"slices"

	"github.com/gardener/gardener/pkg/apis/core"
	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
)

var (
	// validSubnetLogConfigIntervals contain the valid SubnetworkLogConfig AggregationIntervals
	validSubnetLogConfigIntervals = []string{
		"INTERVAL_5_SEC",
		"INTERVAL_30_SEC",
		"INTERVAL_1_MIN",
		"INTERVAL_5_MIN",
		"INTERVAL_10_MIN",
		"INTERVAL_15_MIN",
	}
	validSubnetLogConfigMetadata = []string{
		"CUSTOM_METADATA",
		"EXCLUDE_ALL_METADATA",
		"INCLUDE_ALL_METADATA",
	}
)

// ValidateInfrastructureConfig validates a InfrastructureConfig object.
func ValidateInfrastructureConfig(infra *apisgcp.InfrastructureConfig, nodesCIDR, podsCIDR, servicesCIDR *string, ipFamilies []core.IPFamily, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	networksPath := fldPath.Child("networks")

	if infra.Networks.SubnetWorkers == nil && infra.Networks.SubnetServices != nil {
		allErrs = append(allErrs, field.Forbidden(networksPath.Child("subnetServices"), "subnetServices may only be set in combination with subnetWorkers (bring-your-own-subnet mode)"))
	}

	if infra.Networks.SubnetWorkers != nil {
		allErrs = append(allErrs, validateBYONetworkConfig(&infra.Networks, ipFamilies, networksPath)...)
		return allErrs
	}

	var (
		nodes    cidrvalidation.CIDR
		pods     cidrvalidation.CIDR
		services cidrvalidation.CIDR
	)

	networkingPath := field.NewPath("networking")
	if nodesCIDR != nil {
		nodes = cidrvalidation.NewCIDR(*nodesCIDR, networkingPath.Child("nodes"))
	}
	if podsCIDR != nil {
		pods = cidrvalidation.NewCIDR(*podsCIDR, networkingPath.Child("pods"))
	}
	if servicesCIDR != nil {
		services = cidrvalidation.NewCIDR(*servicesCIDR, networkingPath.Child("services"))
	}

	if len(infra.Networks.Worker) == 0 && len(infra.Networks.Workers) == 0 {
		allErrs = append(allErrs, field.Required(networksPath.Child("workers"), "must specify the network range for the worker network"))
	}

	var workerCIDR cidrvalidation.CIDR
	if infra.Networks.Worker != "" {
		workerCIDR = cidrvalidation.NewCIDR(infra.Networks.Worker, networksPath.Child("worker"))
		allErrs = append(allErrs, cidrvalidation.ValidateCIDRParse(workerCIDR)...)
		allErrs = append(allErrs, cidrvalidation.ValidateCIDRIsCanonical(networksPath.Child("worker"), infra.Networks.Worker)...)
	}
	if infra.Networks.Workers != "" {
		workerCIDR = cidrvalidation.NewCIDR(infra.Networks.Workers, networksPath.Child("workers"))
		allErrs = append(allErrs, cidrvalidation.ValidateCIDRParse(workerCIDR)...)
		allErrs = append(allErrs, cidrvalidation.ValidateCIDRIsCanonical(networksPath.Child("workers"), infra.Networks.Workers)...)
	}

	if infra.Networks.Internal != nil {
		internalCIDR := cidrvalidation.NewCIDR(*infra.Networks.Internal, networksPath.Child("internal"))
		allErrs = append(allErrs, cidrvalidation.ValidateCIDRParse(internalCIDR)...)
		allErrs = append(allErrs, cidrvalidation.ValidateCIDRIsCanonical(networksPath.Child("internal"), *infra.Networks.Internal)...)
		if pods != nil {
			allErrs = append(allErrs, pods.ValidateNotOverlap(internalCIDR)...)
		}
		if services != nil {
			allErrs = append(allErrs, services.ValidateNotOverlap(internalCIDR)...)
		}
		if nodes != nil {
			allErrs = append(allErrs, nodes.ValidateNotOverlap(internalCIDR)...)
		}
		allErrs = append(allErrs, workerCIDR.ValidateNotOverlap(internalCIDR)...)
	}

	if nodes != nil {
		allErrs = append(allErrs, nodes.ValidateSubset(workerCIDR)...)
	}

	if infra.Networks.VPC != nil && len(infra.Networks.VPC.Name) == 0 {
		allErrs = append(allErrs, field.Invalid(networksPath.Child("vpc", "name"), infra.Networks.VPC.Name, "vpc name must not be empty when vpc key is provided"))
	}

	if infra.Networks.VPC != nil && len(infra.Networks.VPC.Name) == 0 && infra.Networks.VPC.CloudRouter != nil {
		allErrs = append(allErrs, field.Invalid(networksPath.Child("vpc", "cloudRouter"), infra.Networks.VPC.CloudRouter, "cloud router can not be configured when the VPC name is not specified"))
	}

	if infra.Networks.VPC != nil && len(infra.Networks.VPC.Name) > 0 {
		allErrs = append(allErrs, validateGcpResourceName(infra.Networks.VPC.Name, networksPath.Child("vpc", "name"))...)

		if infra.Networks.VPC.CloudRouter == nil {
			allErrs = append(allErrs, field.Invalid(networksPath.Child("vpc", "cloudRouter"), infra.Networks.VPC.CloudRouter, "cloud router must be defined when reusing a VPC"))
		}

		if infra.Networks.VPC.CloudRouter != nil {
			if len(infra.Networks.VPC.CloudRouter.Name) == 0 {
				allErrs = append(allErrs, field.Invalid(networksPath.Child("vpc", "cloudRouter", "name"), infra.Networks.VPC.CloudRouter, "cloud router name must be specified when reusing a VPC"))
			} else {
				allErrs = append(allErrs, validateGcpResourceName(infra.Networks.VPC.CloudRouter.Name, networksPath.Child("vpc", "cloudRouter", "name"))...)
			}
		}
	}

	if infra.Networks.FlowLogs != nil {
		allErrs = append(allErrs, validateNetworkFlowLogs(*infra.Networks.FlowLogs, networksPath.Child("flowLogs"))...)
	}

	if infra.Networks.CloudNAT != nil {
		allErrs = append(allErrs, ValidateCloudNatConfig(infra.Networks.CloudNAT, networksPath)...)
	}

	if infra.Networks.MTU != nil {
		mtuPath := networksPath.Child("mtu")
		if infra.Networks.VPC != nil && len(infra.Networks.VPC.Name) > 0 {
			allErrs = append(allErrs, field.Forbidden(mtuPath, "mtu cannot be configured when using an existing VPC"))
		}
		if *infra.Networks.MTU < 1300 || *infra.Networks.MTU > 8896 {
			allErrs = append(allErrs, field.Invalid(mtuPath, *infra.Networks.MTU, "mtu must be between 1300 and 8896"))
		}
	}

	return allErrs
}

// validateBYONetworkConfig validates network configuration when SubnetWorkers is set (BYO subnet mode).
func validateBYONetworkConfig(networks *apisgcp.NetworkConfig, ipFamilies []core.IPFamily, networksPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// VPC.Name is required in BYO mode
	if networks.VPC == nil || len(networks.VPC.Name) == 0 {
		allErrs = append(allErrs, field.Required(networksPath.Child("vpc", "name"), "vpc name must be set when using BYO subnet mode"))
	}

	// CloudRouter must not be set in BYO mode (user manages egress)
	if networks.VPC != nil && networks.VPC.CloudRouter != nil {
		allErrs = append(allErrs, field.Forbidden(networksPath.Child("vpc", "cloudRouter"), "cloudRouter must not be set in BYO subnet mode; manage egress outside of Gardener"))
	}

	// Fields that are only for managed mode must not be set
	if len(networks.Worker) > 0 {
		allErrs = append(allErrs, field.Forbidden(networksPath.Child("worker"), "worker CIDR must not be set in BYO subnet mode; use subnetWorkers instead"))
	}
	if len(networks.Workers) > 0 {
		allErrs = append(allErrs, field.Forbidden(networksPath.Child("workers"), "workers CIDR must not be set in BYO subnet mode; use subnetWorkers instead"))
	}
	if networks.Internal != nil {
		allErrs = append(allErrs, field.Forbidden(networksPath.Child("internal"), "internal subnet must not be set in BYO subnet mode"))
	}
	if networks.CloudNAT != nil {
		allErrs = append(allErrs, field.Forbidden(networksPath.Child("cloudNAT"), "cloudNAT must not be set in BYO subnet mode; manage egress outside of Gardener"))
	}
	if networks.FlowLogs != nil {
		allErrs = append(allErrs, field.Forbidden(networksPath.Child("flowLogs"), "flowLogs must not be set in BYO subnet mode"))
	}
	if networks.MTU != nil {
		allErrs = append(allErrs, field.Forbidden(networksPath.Child("mtu"), "mtu must not be set in BYO subnet mode"))
	}

	// SubnetWorkers must have a non-empty name
	subnetWorkersPath := networksPath.Child("subnetWorkers")
	if len(networks.SubnetWorkers.Name) == 0 {
		allErrs = append(allErrs, field.Required(subnetWorkersPath.Child("name"), "subnetWorkers name must not be empty"))
	}

	// SubnetServices is only valid for dual-stack shoots
	isDualStack := slices.Contains(ipFamilies, core.IPFamilyIPv6)
	subnetServicesPath := networksPath.Child("subnetServices")
	if !isDualStack && networks.SubnetServices != nil {
		allErrs = append(allErrs, field.Forbidden(subnetServicesPath, "subnetServices is only allowed for dual-stack shoots"))
	}
	if isDualStack && networks.SubnetServices == nil {
		allErrs = append(allErrs, field.Required(subnetServicesPath, "subnetServices is required for dual-stack shoots"))
	}
	if isDualStack && networks.SubnetServices != nil {
		if len(networks.SubnetServices.Name) == 0 {
			allErrs = append(allErrs, field.Required(subnetServicesPath.Child("name"), "subnetServices name must not be empty"))
		}
		if networks.SubnetServices.PodSecondaryRangeName != nil {
			allErrs = append(allErrs, field.Forbidden(subnetServicesPath.Child("podSecondaryRangeName"), "podSecondaryRangeName is only valid on subnetWorkers, not subnetServices"))
		}
	}

	podSecondaryRangeNamePath := subnetWorkersPath.Child("podSecondaryRangeName")
	if !isDualStack && networks.SubnetWorkers.PodSecondaryRangeName != nil {
		allErrs = append(allErrs, field.Forbidden(podSecondaryRangeNamePath, "podSecondaryRangeName is only allowed for dual-stack shoots"))
	}
	if isDualStack && networks.SubnetWorkers.PodSecondaryRangeName == nil {
		allErrs = append(allErrs, field.Required(podSecondaryRangeNamePath, "podSecondaryRangeName is required for dual-stack shoots"))
	}
	if isDualStack && networks.SubnetWorkers.PodSecondaryRangeName != nil && len(*networks.SubnetWorkers.PodSecondaryRangeName) == 0 {
		allErrs = append(allErrs, field.Invalid(podSecondaryRangeNamePath, "", "podSecondaryRangeName must not be empty in dual-stack mode"))
	}

	return allErrs
}

func validateNetworkFlowLogs(flowLogs apisgcp.FlowLogs, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if flowLogs.AggregationInterval == nil && flowLogs.FlowSampling == nil && flowLogs.Metadata == nil {
		allErrs = append(allErrs, field.Required(fldPath, "at least one VPC flow log parameter must be specified when VPC flow log section is provided"))
	}

	if flowLogs.AggregationInterval != nil && !slices.Contains(validSubnetLogConfigIntervals, *flowLogs.AggregationInterval) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("aggregationInterval"), *flowLogs.AggregationInterval, validSubnetLogConfigIntervals))
	}

	if flowLogs.Metadata != nil && !slices.Contains(validSubnetLogConfigMetadata, *flowLogs.Metadata) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("metadata"), flowLogs.Metadata, validSubnetLogConfigMetadata))
	}

	if flowLogs.FlowSampling != nil && (*flowLogs.FlowSampling < 0 || *flowLogs.FlowSampling > 1) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("flowSampling"), flowLogs.FlowSampling, "must be between 0 and 1"))
	}

	return allErrs
}

// ValidateCloudNatConfig validates the config of the CloudNat. We intentionally keep the validation light, only
// checking for gotchas (e.g. the port counts having to be powers of two) and obvious errors.
func ValidateCloudNatConfig(config *apisgcp.CloudNAT, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	cloudNatPath := fldPath.Child("cloudNAT")

	if config != nil && config.NatIPNames != nil && len(config.NatIPNames) == 0 {
		allErrs = append(allErrs, field.Invalid(cloudNatPath.Child("natIPNames"), config.NatIPNames, "nat IP names cannot be empty."))
	}

	for idx, natIPName := range config.NatIPNames {
		allErrs = append(allErrs, validateGcpResourceName(natIPName.Name, cloudNatPath.Child("natIPNames").Index(idx).Child("name"))...)
	}

	if config.EnableDynamicPortAllocation {
		if config.EndpointIndependentMapping != nil && config.EndpointIndependentMapping.Enabled {
			// There is no more fitting field.Error (e.g. field.MutuallyExclusive) so we put the blame on 'enableDynamicPortAllocation' and use the error msg
			allErrs = append(allErrs, field.Invalid(cloudNatPath.Child("enableDynamicPortAllocation"), config.EnableDynamicPortAllocation, "dynamic port allocation may not be enabled at the same time as endpoint independent mapping."))
		}
		if config.MaxPortsPerVM != nil && !isPowerOfTwo(*config.MaxPortsPerVM) {
			allErrs = append(allErrs, field.Invalid(cloudNatPath.Child("maxPortsPerVM"), config.MaxPortsPerVM, "maxPortsPerVM must be a power of two."))
		}
		if config.MinPortsPerVM != nil && !isPowerOfTwo(*config.MinPortsPerVM) {
			allErrs = append(allErrs, field.Invalid(cloudNatPath.Child("minPortsPerVM"), config.MinPortsPerVM, "minPortsPerVM must be a power of two if dynamic port allocation is enabled."))
		}
		if config.MaxPortsPerVM != nil && config.MinPortsPerVM != nil && *config.MinPortsPerVM > *config.MaxPortsPerVM {
			allErrs = append(allErrs, field.Invalid(cloudNatPath.Child("minPortsPerVM"), config.MinPortsPerVM, "minPortsPerVM may not be greater than maxPortsPerVM."))
		}
	} else {
		if config.MaxPortsPerVM != nil {
			allErrs = append(allErrs, field.Invalid(cloudNatPath.Child("maxPortsPerVM"), config.MinPortsPerVM, "maxPortsPerVM are only configurable if dynamic port allocation is enabled."))
		}
	}

	return allErrs
}

func isPowerOfTwo(integer int32) bool {
	// Compare the binary representation of the given positive integer with its predecessor, e.g. '11011' (27) and '11010' (26).
	// They will share (at least) the leading '1' resulting in the union of them representing a number greater than zero, unless the given one is a power of two.
	// Note: Also works for zero.
	return integer&(integer-1) == 0
}

// ValidateInfrastructureConfigUpdate validates a InfrastructureConfig object.
func ValidateInfrastructureConfigUpdate(oldConfig, newConfig *apisgcp.InfrastructureConfig, fldPath *field.Path) field.ErrorList {
	var (
		allErrs      = field.ErrorList{}
		networksPath = fldPath.Child("networks")
		vpcPath      = networksPath.Child("vpc")
	)

	// Switching between BYO and managed mode is not allowed
	oldBYO := oldConfig.Networks.SubnetWorkers != nil
	newBYO := newConfig.Networks.SubnetWorkers != nil
	if oldBYO != newBYO {
		allErrs = append(allErrs, field.Forbidden(networksPath.Child("subnetWorkers"), "cannot switch between BYO subnet mode and managed mode"))
		return allErrs
	}

	// SubnetWorkers and SubnetServices are immutable once set
	if oldBYO {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newConfig.Networks.SubnetWorkers, oldConfig.Networks.SubnetWorkers, networksPath.Child("subnetWorkers"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newConfig.Networks.SubnetServices, oldConfig.Networks.SubnetServices, networksPath.Child("subnetServices"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newConfig.Networks.VPC, oldConfig.Networks.VPC, vpcPath)...)
		return allErrs
	}

	oldVPC := oldConfig.Networks.VPC
	newVPC := newConfig.Networks.VPC

	if oldVPC != nil && newVPC == nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newVPC, oldVPC, vpcPath)...)
	}

	if oldVPC != nil && newVPC != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newVPC.Name, oldVPC.Name, vpcPath.Child("name"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newVPC.CloudRouter, oldVPC.CloudRouter, vpcPath.Child("cloudRouter"))...)

		// Allow adding an internal subnet if it didn't exist before, but prevent changing an existing one.
		if oldConfig.Networks.Internal != nil {
			allErrs = append(allErrs, apivalidation.ValidateImmutableField(newConfig.Networks.Internal, oldConfig.Networks.Internal, networksPath.Child("internal"))...)
		}
	}

	newWorkerCIDR := newConfig.Networks.Worker
	newWorker := cidrvalidation.NewCIDR(newWorkerCIDR, networksPath.Child("worker"))
	if len(newConfig.Networks.Workers) > 0 {
		newWorkerCIDR = newConfig.Networks.Workers
		newWorker = cidrvalidation.NewCIDR(newWorkerCIDR, networksPath.Child("workers"))
	}

	oldWorkerCIDR := oldConfig.Networks.Worker
	oldWorker := cidrvalidation.NewCIDR(oldWorkerCIDR, networksPath.Child("worker"))
	if len(oldConfig.Networks.Workers) > 0 {
		oldWorkerCIDR = oldConfig.Networks.Workers
		oldWorker = cidrvalidation.NewCIDR(oldWorkerCIDR, networksPath.Child("workers"))
	}
	if len(newWorker.ValidateSubset(oldWorker)) > 0 {
		allErrs = append(allErrs, field.Invalid(newWorker.GetFieldPath(), newWorker.GetCIDR(), "worker CIDR blocks can only be expanded"))
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newConfig.Networks.MTU, oldConfig.Networks.MTU, networksPath.Child("mtu"))...)

	return allErrs
}
