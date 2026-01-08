// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	"time"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
)

var _ = Describe("Scheme", func() {
	Describe("WorkloadIdentityConfigFromBytes", func() {
		It("should successfully parse WorkloadIdentityConfig", func() {
			raw := []byte(`apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
kind: WorkloadIdentityConfig
projectID: "my-project"
credentialsConfig:
  universe_domain: "googleapis.com"
  type: "external_account"
  audience: "//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider"
  subject_token_type: "urn:ietf:params:oauth:token-type:jwt"
  token_url: "https://sts.googleapis.com/v1/token/new"
`)
			config, err := helper.WorkloadIdentityConfigFromBytes(raw)
			Expect(err).ToNot(HaveOccurred())
			Expect(config).ToNot(BeNil())
			Expect(config.ProjectID).To(Equal("my-project"))
		})

		It("should fail to parse WorkloadIdentityConfig due to nil config", func() {
			config, err := helper.WorkloadIdentityConfigFromBytes(nil)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("cannot parse WorkloadIdentityConfig from empty config"))
			Expect(config).To(BeNil())
		})

		It("should fail to parse WorkloadIdentityConfig due to empty config", func() {
			config, err := helper.WorkloadIdentityConfigFromBytes([]byte{})
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("cannot parse WorkloadIdentityConfig from empty config"))
			Expect(config).To(BeNil())
		})

		It("should fail to parse WorkloadIdentityConfig due to unknown field", func() {
			raw := []byte(`apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
kind: WorkloadIdentityConfig
projectID: "my-project"
credentialsConfig:
  universe_domain: "googleapis.com"
  type: "external_account"
  audience: "//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider"
  subject_token_type: "urn:ietf:params:oauth:token-type:jwt"
  token_url: "https://sts.googleapis.com/v1/token/new"
additionalField: additionalValue
`)
			config, err := helper.WorkloadIdentityConfigFromBytes(raw)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("strict decoding error: unknown field \"additionalField\""))
			Expect(config).To(BeNil())
		})

		It("should fail to parse WorkloadIdentityConfig due to missing apiVersion", func() {
			raw := []byte(`kind: WorkloadIdentityConfig
projectID: "my-project"
credentialsConfig:
  universe_domain: "googleapis.com"
  type: "external_account"
  audience: "//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider"
  subject_token_type: "urn:ietf:params:oauth:token-type:jwt"
  token_url: "https://sts.googleapis.com/v1/token/new"
`)
			config, err := helper.WorkloadIdentityConfigFromBytes(raw)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("Object 'apiVersion' is missing in")))
			Expect(config).To(BeNil())
		})

		It("should fail to parse WorkloadIdentityConfig due to unsupported apiVersion", func() {
			raw := []byte(`apiVersion: gcp.provider.extensions.gardener.cloud/v0
kind: WorkloadIdentityConfig
projectID: "my-project"
credentialsConfig:
  universe_domain: "googleapis.com"
  type: "external_account"
  audience: "//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider"
  subject_token_type: "urn:ietf:params:oauth:token-type:jwt"
  token_url: "https://sts.googleapis.com/v1/token/new"
`)
			config, err := helper.WorkloadIdentityConfigFromBytes(raw)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("no kind \"WorkloadIdentityConfig\" is registered for version \"gcp.provider.extensions.gardener.cloud/v0\" in scheme")), err.Error())
			Expect(config).To(BeNil())
		})

		It("should fail to parse WorkloadIdentityConfig due unregistered kind", func() {
			raw := []byte(`apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
kind: FooBar
projectID: "my-project"
credentialsConfig:
  universe_domain: "googleapis.com"
  type: "external_account"
  audience: "//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider"
  subject_token_type: "urn:ietf:params:oauth:token-type:jwt"
  token_url: "https://sts.googleapis.com/v1/token/new"
`)
			config, err := helper.WorkloadIdentityConfigFromBytes(raw)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("no kind \"FooBar\" is registered for version \"gcp.provider.extensions.gardener.cloud/v1alpha1\" in scheme")))
			Expect(config).To(BeNil())
		})
	})

	Describe("WorkloadIdentityConfigFromRaw", func() {
		It("should fail to parse WorkloadIdentityConfig due to nil raw", func() {
			config, err := helper.WorkloadIdentityConfigFromRaw(nil)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("cannot parse WorkloadIdentityConfig from empty RawExtension"))
			Expect(config).To(BeNil())
		})

		It("should fail to parse WorkloadIdentityConfig due to nil raw", func() {
			config, err := helper.WorkloadIdentityConfigFromRaw(&runtime.RawExtension{Raw: nil})
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("cannot parse WorkloadIdentityConfig from empty RawExtension"))
			Expect(config).To(BeNil())
		})

		It("should successfully parse WorkloadIdentityConfig", func() {
			raw := &runtime.RawExtension{Raw: []byte(`apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
kind: WorkloadIdentityConfig
projectID: "my-project"
credentialsConfig:
  universe_domain: "googleapis.com"
  type: "external_account"
  audience: "//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider"
  subject_token_type: "urn:ietf:params:oauth:token-type:jwt"
  token_url: "https://sts.googleapis.com/v1/token/new"
`)}
			config, err := helper.WorkloadIdentityConfigFromRaw(raw)
			Expect(err).ToNot(HaveOccurred())
			Expect(config.ProjectID).To(Equal("my-project"))
		})
	})

	Describe("BackupBucketConfigFromBackupBucket", func() {
		It("should successfully parse BackupBucketConfig", func() {
			raw := []byte(`apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
kind: BackupBucketConfig
immutability:
  retentionType: bucket
  retentionPeriod: "24h"
  locked: true
endpoint: "https://storage.googleapis.com"`)

			bb := &extensionsv1alpha1.BackupBucket{}
			bb.Spec.ProviderConfig = &runtime.RawExtension{Raw: raw}

			config, err := helper.BackupBucketConfigFromBackupBucket(bb)
			Expect(err).ToNot(HaveOccurred())
			Expect(config).ToNot(BeNil())
			Expect(config.Immutability).ToNot(BeNil())
			Expect(config.Immutability.RetentionType).To(Equal("bucket"))
			Expect(config.Immutability.RetentionPeriod.Duration).To(Equal(24 * time.Hour))
			Expect(config.Immutability.Locked).To(BeTrue())
			Expect(config.Endpoint).ToNot(BeNil())
			Expect(*config.Endpoint).To(Equal("https://storage.googleapis.com"))
		})

		It("should fail due to nil ProviderConfig", func() {
			bb := &extensionsv1alpha1.BackupBucket{}
			bb.Spec.ProviderConfig = nil

			config, err := helper.BackupBucketConfigFromBackupBucket(bb)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("provider config is not set on the backupbucket resource"))
			Expect(config).To(BeNil())
		})

		It("should fail due to nil Raw", func() {
			bb := &extensionsv1alpha1.BackupBucket{}
			bb.Spec.ProviderConfig = &runtime.RawExtension{Raw: nil}

			config, err := helper.BackupBucketConfigFromBackupBucket(bb)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("provider config is not set on the backupbucket resource"))
			Expect(config).To(BeNil())
		})

		It("should fail to parse BackupBucketConfig due to unknown field", func() {
			raw := []byte(`apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
kind: BackupBucketConfig
immutability:
  retentionType: bucket
  retentionPeriod: "24h"
additionalField: additionalValue`)

			bb := &extensionsv1alpha1.BackupBucket{}
			bb.Spec.ProviderConfig = &runtime.RawExtension{Raw: raw}

			config, err := helper.BackupBucketConfigFromBackupBucket(bb)
			Expect(err).To(HaveOccurred())
			Expect(config).To(BeNil())
		})

		It("should fail to parse BackupBucketConfig due to missing apiVersion", func() {
			raw := []byte(`kind: BackupBucketConfig
immutability:
  retentionType: bucket
  retentionPeriod: "24h"`)

			bb := &extensionsv1alpha1.BackupBucket{}
			bb.Spec.ProviderConfig = &runtime.RawExtension{Raw: raw}

			config, err := helper.BackupBucketConfigFromBackupBucket(bb)
			Expect(err).To(HaveOccurred())
			Expect(config).To(BeNil())
		})

		It("should fail to parse BackupBucketConfig due to unsupported apiVersion", func() {
			raw := []byte(`apiVersion: gcp.provider.extensions.gardener.cloud/v0
kind: BackupBucketConfig
immutability:
  retentionType: bucket
  retentionPeriod: "24h"`)

			bb := &extensionsv1alpha1.BackupBucket{}
			bb.Spec.ProviderConfig = &runtime.RawExtension{Raw: raw}

			config, err := helper.BackupBucketConfigFromBackupBucket(bb)
			Expect(err).To(HaveOccurred())
			Expect(config).To(BeNil())
		})

		It("should fail to parse BackupBucketConfig due to unregistered kind", func() {
			raw := []byte(`apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
kind: FooBar
immutability:
  retentionType: bucket
  retentionPeriod: "24h"`)

			bb := &extensionsv1alpha1.BackupBucket{}
			bb.Spec.ProviderConfig = &runtime.RawExtension{Raw: raw}

			config, err := helper.BackupBucketConfigFromBackupBucket(bb)
			Expect(err).To(HaveOccurred())
			Expect(config).To(BeNil())
		})
	})
})
