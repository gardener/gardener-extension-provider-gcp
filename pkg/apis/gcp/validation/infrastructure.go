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

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
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
		allErrs = append(allErrs, cidrvalidation.ValidateCIDROverlap([]cidrvalidation.CIDR{pods, services}, []cidrvalidation.CIDR{internalCIDR}, false)...)
		allErrs = append(allErrs, cidrvalidation.ValidateCIDROverlap([]cidrvalidation.CIDR{nodes}, []cidrvalidation.CIDR{internalCIDR}, false)...)
		allErrs = append(allErrs, cidrvalidation.ValidateCIDROverlap([]cidrvalidation.CIDR{workerCIDR}, []cidrvalidation.CIDR{internalCIDR}, false)...)
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

	return allErrs
}

// ValidateInfrastructureConfigUpdate validates a InfrastructureConfig object.
func ValidateInfrastructureConfigUpdate(oldConfig, newConfig *apisgcp.InfrastructureConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newConfig.Networks, oldConfig.Networks, fldPath.Child("networks"))...)

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
