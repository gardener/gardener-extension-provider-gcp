// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gcp_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

	Describe("#IsWorkloadIdentitySecret", func() {
		var secret *corev1.Secret

		BeforeEach(func() {
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"security.gardener.cloud/purpose":                   "workload-identity-token-requestor",
						"workloadidentity.security.gardener.cloud/provider": "gcp",
					},
				},
			}
		})

		It("should return false if labels are nil", func() {
			secret.Labels = nil
			Expect(gcp.IsWorkloadIdentitySecret(secret)).To(BeFalse())
		})

		It("should return false if purpose label is missing", func() {
			delete(secret.Labels, "security.gardener.cloud/purpose")
			Expect(gcp.IsWorkloadIdentitySecret(secret)).To(BeFalse())
		})

		It("should return false if purpose label is not workload-identity-token-requestor", func() {
			secret.Labels["security.gardener.cloud/purpose"] = "something-else"
			Expect(gcp.IsWorkloadIdentitySecret(secret)).To(BeFalse())
		})

		It("should return false if provider label is missing", func() {
			delete(secret.Labels, "workloadidentity.security.gardener.cloud/provider")
			Expect(gcp.IsWorkloadIdentitySecret(secret)).To(BeFalse())
		})

		It("should return false if provider label is not gcp", func() {
			secret.Labels["workloadidentity.security.gardener.cloud/provider"] = "other-provider"
			Expect(gcp.IsWorkloadIdentitySecret(secret)).To(BeFalse())
		})

		It("should return true if all required labels are set correctly", func() {
			Expect(gcp.IsWorkloadIdentitySecret(secret)).To(BeTrue())
		})

	})
})
