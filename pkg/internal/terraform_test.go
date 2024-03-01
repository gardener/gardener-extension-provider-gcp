// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

var _ = Describe("Terraform", func() {
	var (
		projectID          string
		serviceAccountData []byte
		serviceAccount     *gcp.ServiceAccount
	)
	BeforeEach(func() {
		projectID = "project"
		serviceAccountData = []byte(fmt.Sprintf(`{"project_id": "%s"}`, projectID))
		serviceAccount = &gcp.ServiceAccount{ProjectID: projectID, Raw: serviceAccountData}
	})

	Describe("#TerraformerVariablesEnvironmentFromServiceAccount", func() {
		It("should correctly create the variables environment", func() {
			variables, err := TerraformerVariablesEnvironmentFromServiceAccount(serviceAccount)

			Expect(err).NotTo(HaveOccurred())
			Expect(variables).To(Equal(map[string]string{
				TerraformVarServiceAccount: fmt.Sprintf(`{"project_id":"%s"}`, projectID),
			}))
		})
	})
})
