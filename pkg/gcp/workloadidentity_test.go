// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gcp_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

var _ = Describe("WorkloadIdentity", func() {
	var _ = Describe("#SetWorkloadIdentityFeatures", func() {
		const mountPath = gcp.WorkloadIdentityMountPath
		var (
			data   map[string][]byte
			config []byte
		)

		BeforeEach(func() {
			data = map[string][]byte{}
			config = []byte(`apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
kind: WorkloadIdentityConfig
projectID: test-proj
credentialsConfig:
  universe_domain: "googleapis.com"
  type: "external_account"
  audience: "//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider"
  subject_token_type: "urn:ietf:params:oauth:token-type:jwt"
  token_url: "https://sts.googleapis.com/v1/token"
  service_account_impersonation_url: "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/SERVICE_ACCOUNT_EMAIL:generateAccessToken"
`)
			data["config"] = config
		})

		It("should fail when 'config' key is not set", func() {
			delete(data, "config")
			Expect(gcp.SetWorkloadIdentityFeatures(data, mountPath)).To(And(
				Not(Succeed()),
				MatchError(ContainSubstring("'config' key is missing in the map")),
			))
		})

		It("should fail when 'config' key is set to empty value", func() {
			data["config"] = []byte{}
			Expect(gcp.SetWorkloadIdentityFeatures(data, mountPath)).To(And(
				Not(Succeed()),
				MatchError(ContainSubstring("could not decode 'config' as WorkloadIdentityConfig")),
			))
		})

		It("should fail when 'config' key is set to invalid value", func() {
			data["config"] = []byte("invalid-value")

			Expect(gcp.SetWorkloadIdentityFeatures(data, mountPath)).To(And(
				Not(Succeed()),
				MatchError(ContainSubstring("could not decode 'config' as WorkloadIdentityConfig")),
			))
		})

		It("should successfully set workload identity features", func() {
			expectedCredentialsConfig := `{"audience":"//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider","credential_source":{"file":"` + mountPath + `/token","format":{"type":"text"}},"service_account_impersonation_url":"https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/SERVICE_ACCOUNT_EMAIL:generateAccessToken","subject_token_type":"urn:ietf:params:oauth:token-type:jwt","token_url":"https://sts.googleapis.com/v1/token","type":"external_account","universe_domain":"googleapis.com"}`
			Expect(gcp.SetWorkloadIdentityFeatures(data, mountPath)).To(Succeed())

			Expect(data).To(HaveKeyWithValue("projectID", []byte("test-proj")))
			Expect(data).To(HaveKey("credentialsConfig"))
			Expect(string(data["credentialsConfig"])).To(Equal(expectedCredentialsConfig), string(data["credentialsConfig"]))
		})

	})
})
