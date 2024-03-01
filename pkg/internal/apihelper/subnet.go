// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apihelper

import (
	"fmt"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
)

// FindSubnetForPurpose takes a list of subnets and tries to find the first entry
// whose purpose matches with the given purpose. If no such entry is found then an error will be
// returned.
func FindSubnetForPurpose(subnets []gcp.Subnet, purpose gcp.SubnetPurpose) (*gcp.Subnet, error) {
	for _, subnet := range subnets {
		if subnet.Purpose == purpose {
			return &subnet, nil
		}
	}
	return nil, fmt.Errorf("no subnet with purpose %q found", purpose)
}
