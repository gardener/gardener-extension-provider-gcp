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
