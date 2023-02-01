// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package validation

import (
	"reflect"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"

	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"
	"k8s.io/apimachinery/pkg/util/validation/field"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
)

// ValidateInfrastructureConfig validates a InfrastructureConfig object.
func ValidateInfrastructureConfig(infra *apisgcp.InfrastructureConfig, nodesCIDR, podsCIDR, servicesCIDR *string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	var (
		nodes                    cidrvalidation.CIDR
		pods                     cidrvalidation.CIDR
		services                 cidrvalidation.CIDR
		aggregationIntervalArray = []string{"INTERVAL_5_SEC", "INTERVAL_30_SEC", "INTERVAL_1_MIN", "INTERVAL_5_MIN", "INTERVAL_15_MIN"}
		metadata                 = []string{"INCLUDE_ALL_METADATA"}
	)

	if nodesCIDR != nil {
		nodes = cidrvalidation.NewCIDR(*nodesCIDR, nil)
	}
	if podsCIDR != nil {
		pods = cidrvalidation.NewCIDR(*podsCIDR, nil)
	}
	if servicesCIDR != nil {
		services = cidrvalidation.NewCIDR(*servicesCIDR, nil)
	}

	networksPath := fldPath.Child("networks")
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
		if infra.Networks.VPC.CloudRouter == nil {
			allErrs = append(allErrs, field.Invalid(networksPath.Child("vpc", "cloudRouter"), infra.Networks.VPC.CloudRouter, "cloud router must be defined when reusing a VPC"))
		}

		if infra.Networks.VPC.CloudRouter != nil && len(infra.Networks.VPC.CloudRouter.Name) == 0 {
			allErrs = append(allErrs, field.Invalid(networksPath.Child("vpc", "cloudRouter", "name"), infra.Networks.VPC.CloudRouter, "cloud router name must be specified when reusing a VPC"))
		}
	}

	if infra.Networks.FlowLogs != nil {
		if infra.Networks.FlowLogs.AggregationInterval == nil && infra.Networks.FlowLogs.FlowSampling == nil && infra.Networks.FlowLogs.Metadata == nil {
			allErrs = append(allErrs, field.Required(networksPath.Child("flowLogs"), "at least one VPC flow log parameter must be specified when VPC flow log section is provided"))
		}
		if infra.Networks.FlowLogs.AggregationInterval != nil {
			validValue := findElement(aggregationIntervalArray, *infra.Networks.FlowLogs.AggregationInterval)
			if !validValue {
				allErrs = append(allErrs, field.NotSupported(networksPath.Child("flowLogs", "aggregationInterval"), infra.Networks.FlowLogs.AggregationInterval, aggregationIntervalArray))
			}
		}
		if infra.Networks.FlowLogs.Metadata != nil {
			validValue := findElement(metadata, *infra.Networks.FlowLogs.Metadata)
			if !validValue {
				allErrs = append(allErrs, field.NotSupported(networksPath.Child("flowLogs", "metadata"), infra.Networks.FlowLogs.Metadata, metadata))
			}
		}
		if infra.Networks.FlowLogs.FlowSampling != nil {
			if *infra.Networks.FlowLogs.FlowSampling < 0 || *infra.Networks.FlowLogs.FlowSampling > 1 {
				allErrs = append(allErrs, field.Invalid(networksPath.Child("flowLogs", "flowSampling"), infra.Networks.FlowLogs.FlowSampling, "must contain a valid value"))
			}
		}
	}

	if infra.Networks.CloudNAT != nil && infra.Networks.CloudNAT.NatIPNames != nil && len(infra.Networks.CloudNAT.NatIPNames) == 0 {
		allErrs = append(allErrs, field.Invalid(networksPath.Child("cloudNAT", "natIPNames"), infra.Networks.CloudNAT.NatIPNames, "nat IP names cannot be empty"))
	}

	return allErrs
}

// ValidateInfrastructureConfigUpdate validates a InfrastructureConfig object.
func ValidateInfrastructureConfigUpdate(oldConfig, newConfig *apisgcp.InfrastructureConfig, fldPath *field.Path) field.ErrorList {
	var (
		allErrs      = field.ErrorList{}
		networksPath = fldPath.Child("networks")
		vpcPath      = networksPath.Child("vpc")
	)

	oldVPC := oldConfig.Networks.VPC
	newVPC := newConfig.Networks.VPC

	if oldVPC != nil && newVPC == nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newVPC, oldVPC, vpcPath)...)
	}

	if oldVPC != nil && newVPC != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newVPC.Name, oldVPC.Name, vpcPath.Child("name"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newVPC.CloudRouter, oldVPC.CloudRouter, vpcPath.Child("cloudRouter"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newConfig.Networks.Internal, oldConfig.Networks.Internal, networksPath.Child("internal"))...)
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

	return allErrs
}

// FindElement takes a slice and an item and tries to find the item in the slice.
// if item is found, true is returned.
func findElement(slice interface{}, item interface{}) bool {
	s := reflect.ValueOf(slice)
	if s.Kind() != reflect.Slice {
		panic("Invalid data type")
	}
	for i := 0; i < s.Len(); i++ {
		if s.Index(i).Interface() == item {
			return true
		}
	}
	return false
}
