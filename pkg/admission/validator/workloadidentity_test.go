// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"
	"regexp"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/admission/validator"
	gcpapi "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	gcpapiv1alpha1 "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
)

var _ = Describe("WorkloadIdentity validator", func() {
	Describe("#Validate", func() {
		var (
			workloadIdentityValidator extensionswebhook.Validator
			workloadIdentity          *securityv1alpha1.WorkloadIdentity
			ctx                       = context.Background()
		)

		BeforeEach(func() {
			workloadIdentity = &securityv1alpha1.WorkloadIdentity{
				Spec: securityv1alpha1.WorkloadIdentitySpec{
					Audiences: []string{"foo"},
					TargetSystem: securityv1alpha1.TargetSystem{
						Type: "gcp",
						ProviderConfig: &runtime.RawExtension{
							Raw: []byte(`
apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
kind: WorkloadIdentityConfig
projectID: "foo-valid"
credentialsConfig:
  "universe_domain": "googleapis.com"
  "type": "external_account"
  "audience": "//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider"
  "subject_token_type": "urn:ietf:params:oauth:token-type:jwt"
  "token_url": "https://sts.googleapis.com/v1/token"
  "service_account_impersonation_url": "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/foo@bar.example:generateAccessToken"
`),
						},
					},
				},
			}
			scheme := runtime.NewScheme()
			Expect(securityv1alpha1.AddToScheme(scheme)).To(Succeed())
			Expect(gcpapi.AddToScheme(scheme)).To(Succeed())
			Expect(gcpapiv1alpha1.AddToScheme(scheme)).To(Succeed())

			workloadIdentityValidator = validator.NewWorkloadIdentityValidator(
				serializer.NewCodecFactory(scheme, serializer.EnableStrict).UniversalDecoder(),
				[]string{"https://sts.googleapis.com/v1/token", "https://sts.googleapis.com/v1/token/new"},
				[]*regexp.Regexp{regexp.MustCompile(`^https://iamcredentials\.googleapis\.com/v1/projects/-/serviceAccounts/.+:generateAccessToken$`)},
			)
		})

		It("should skip validation if workload identity is not of type 'gcp'", func() {
			workloadIdentity.Spec.TargetSystem.Type = "foo"
			Expect(workloadIdentityValidator.Validate(ctx, workloadIdentity, nil)).To(Succeed())
		})

		It("should successfully validate the creation of a workload identity", func() {
			Expect(workloadIdentityValidator.Validate(ctx, workloadIdentity, nil)).To(Succeed())
		})

		It("should successfully validate the update of a workload identity", func() {
			newWorkloadIdentity := workloadIdentity.DeepCopy()
			newWorkloadIdentity.Spec.TargetSystem.ProviderConfig.Raw = []byte(`
apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
kind: WorkloadIdentityConfig
projectID: "foo-valid"
credentialsConfig:
  universe_domain: "googleapis.com"
  type: "external_account"
  audience: "//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider"
  subject_token_type: "urn:ietf:params:oauth:token-type:jwt"
  token_url: "https://sts.googleapis.com/v1/token/new"
`)
			Expect(workloadIdentityValidator.Validate(ctx, newWorkloadIdentity, workloadIdentity)).To(Succeed())
		})

		It("should not allow changing the projectID", func() {
			newWorkloadIdentity := workloadIdentity.DeepCopy()
			newWorkloadIdentity.Spec.TargetSystem.ProviderConfig.Raw = []byte(`
apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
kind: WorkloadIdentityConfig
projectID: "foo-valid-new"
credentialsConfig:
  universe_domain: "googleapis.com"
  type: "external_account"
  audience: "//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider"
  subject_token_type: "urn:ietf:params:oauth:token-type:jwt"
  token_url: "https://sts.googleapis.com/v1/token"
`)
			err := workloadIdentityValidator.Validate(ctx, newWorkloadIdentity, workloadIdentity)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("validation of target system's configuration failed: spec.targetSystem.providerConfig.projectID: Invalid value: \"foo-valid-new\": field is immutable"))
		})

		It("should not allow changing forbidden token_url", func() {
			newWorkloadIdentity := workloadIdentity.DeepCopy()
			newWorkloadIdentity.Spec.TargetSystem.ProviderConfig.Raw = []byte(`
apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
kind: WorkloadIdentityConfig
projectID: "foo-valid"
credentialsConfig:
  universe_domain: "googleapis.com"
  type: "external_account"
  audience: "//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider"
  subject_token_type: "urn:ietf:params:oauth:token-type:jwt"
  token_url: "https://sts.googleapis.com/v1/token-forbidden"
`)
			err := workloadIdentityValidator.Validate(ctx, newWorkloadIdentity, workloadIdentity)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(`validation of target system's configuration failed: spec.targetSystem.providerConfig.credentialsConfig.token_url: Unsupported value: "https://sts.googleapis.com/v1/token-forbidden": supported values: "https://sts.googleapis.com/v1/token", "https://sts.googleapis.com/v1/token/new"`))
		})

		It("should not allow changing forbidden service_account_impersonation_url", func() {
			newWorkloadIdentity := workloadIdentity.DeepCopy()
			newWorkloadIdentity.Spec.TargetSystem.ProviderConfig.Raw = []byte(`
apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
kind: WorkloadIdentityConfig
projectID: "foo-valid"
credentialsConfig:
  universe_domain: "googleapis.com"
  type: "external_account"
  audience: "//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider"
  subject_token_type: "urn:ietf:params:oauth:token-type:jwt"
  token_url: "https://sts.googleapis.com/v1/token"
  service_account_impersonation_url: "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/foo@bar.example:generateAccessTokeninvalid"
`)
			err := workloadIdentityValidator.Validate(ctx, newWorkloadIdentity, workloadIdentity)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(`validation of target system's configuration failed: spec.targetSystem.providerConfig.credentialsConfig.service_account_impersonation_url: Invalid value: "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/foo@bar.example:generateAccessTokeninvalid": should match one of the allowed regular expressions: ^https://iamcredentials\.googleapis\.com/v1/projects/-/serviceAccounts/.+:generateAccessToken$`))
		})
	})
})
