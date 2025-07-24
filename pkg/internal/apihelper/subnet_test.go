// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apihelper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	. "github.com/gardener/gardener-extension-provider-gcp/pkg/internal/apihelper"
)

var _ = Describe("Subnet", func() {
	DescribeTable("#FindSubnetForPurpose",
		func(subnets []gcp.Subnet, purpose gcp.SubnetPurpose, expectedSubnet *gcp.Subnet) {
			subnet, err := FindSubnetForPurpose(subnets, purpose)

			if expectedSubnet != nil {
				Expect(*subnet).To(Equal(*expectedSubnet))
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(subnet).To(BeNil())
				Expect(err).To(HaveOccurred())
			}
		},

		Entry("list is nil", nil, gcp.PurposeInternal, nil),
		Entry("empty list", []gcp.Subnet{}, gcp.PurposeInternal, nil),
		Entry("entry not found", []gcp.Subnet{{Name: "bar", Purpose: gcp.SubnetPurpose("baz")}}, gcp.PurposeInternal, nil),
		Entry("entry exists", []gcp.Subnet{{Name: "bar", Purpose: gcp.PurposeInternal}}, gcp.PurposeInternal, &gcp.Subnet{Name: "bar", Purpose: gcp.PurposeInternal}),
	)
})
