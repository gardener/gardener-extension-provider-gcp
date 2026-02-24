// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
)

var _ = Describe("BackupBucket", func() {
	var fldPath *field.Path

	BeforeEach(func() {
		fldPath = field.NewPath("spec")
	})

	Describe("ValidateBackupBucketConfig", func() {
		DescribeTable("validation cases",
			func(config *apisgcp.BackupBucketConfig, wantErr bool, errMsg string) {
				errs := ValidateBackupBucketConfig(config, []string{"https://storage.me-central2.rep.googleapis.com"}, fldPath)
				if wantErr {
					Expect(errs).NotTo(BeEmpty())
					Expect(errs[0].Error()).To(ContainSubstring(errMsg))
				} else {
					Expect(errs).To(BeEmpty())
				}
			},
			Entry("valid config",
				&apisgcp.BackupBucketConfig{
					Immutability: &apisgcp.ImmutableConfig{
						RetentionType:   "bucket",
						RetentionPeriod: metav1.Duration{Duration: 24 * time.Hour},
					},
				}, false, ""),
			Entry("missing retentionType",
				&apisgcp.BackupBucketConfig{
					Immutability: &apisgcp.ImmutableConfig{
						RetentionType:   "",
						RetentionPeriod: metav1.Duration{Duration: 1 * time.Hour},
					},
				}, true, "must be 'bucket'"),
			Entry("invalid retentionType",
				&apisgcp.BackupBucketConfig{
					Immutability: &apisgcp.ImmutableConfig{
						RetentionType:   "invalid",
						RetentionPeriod: metav1.Duration{Duration: 1 * time.Hour},
					},
				}, true, "must be 'bucket'"),
			Entry("non-positive retentionPeriod",
				&apisgcp.BackupBucketConfig{
					Immutability: &apisgcp.ImmutableConfig{
						RetentionType:   "bucket",
						RetentionPeriod: metav1.Duration{Duration: 0},
					},
				}, true, "must be a positive duration greater than 24h"),
			Entry("negative retentionPeriod",
				&apisgcp.BackupBucketConfig{
					Immutability: &apisgcp.ImmutableConfig{
						RetentionType:   "bucket",
						RetentionPeriod: metav1.Duration{Duration: -1 * time.Hour},
					},
				}, true, "must be a positive duration greater than 24h"),
			Entry("empty retentionPeriod",
				&apisgcp.BackupBucketConfig{
					Immutability: &apisgcp.ImmutableConfig{
						RetentionType:   "bucket",
						RetentionPeriod: metav1.Duration{},
					},
				}, true, "must be a positive duration greater than 24h"),
			Entry("retentionPeriod less than 24 hours",
				&apisgcp.BackupBucketConfig{
					Immutability: &apisgcp.ImmutableConfig{
						RetentionType:   "bucket",
						RetentionPeriod: metav1.Duration{Duration: 23 * time.Hour},
					},
				}, true, "must be a positive duration greater than 24h"),
			Entry("endpointOverride URL invalid",
				&apisgcp.BackupBucketConfig{
					Store: &apisgcp.Store{
						EndpointOverride: ptr.To("https://gardener.cloud:invalidport"),
					},
				}, true, "invalid URL"),
			Entry("endpointOverride URL with http scheme",
				&apisgcp.BackupBucketConfig{
					Store: &apisgcp.Store{
						EndpointOverride: ptr.To("http://storage.googleapis.com"),
					},
				}, true, "must use https scheme"),
			Entry("endpointOverride URL with no scheme",
				&apisgcp.BackupBucketConfig{
					Store: &apisgcp.Store{
						EndpointOverride: ptr.To("storage.googleapis.com"),
					},
				}, true, "must use https scheme"),
			Entry("endpointOverride URL not explicitly allowed",
				&apisgcp.BackupBucketConfig{
					Store: &apisgcp.Store{
						EndpointOverride: ptr.To("https://not.explicitly.allowed.endpointurl"),
					},
				}, true, "endpointOverride: Unsupported value"),
			Entry("endpointOverride URL valid and explicitly allowed",
				&apisgcp.BackupBucketConfig{
					Store: &apisgcp.Store{
						EndpointOverride: ptr.To("https://storage.me-central2.rep.googleapis.com"),
					},
				}, false, ""),
		)
	})

	Describe("ValidateBackupBucketConfigUpdate", func() {
		DescribeTable("Valid update scenarios",
			func(oldConfig, newConfig apisgcp.BackupBucketConfig) {
				Expect(ValidateBackupBucketConfigUpdate(&oldConfig, &newConfig, fldPath).ToAggregate()).To(Succeed())
			},
			Entry("Immutable settings unchanged",
				generateBackupBucketConfig("bucket", ptr.To(time.Hour*96), false, true),
				generateBackupBucketConfig("bucket", ptr.To(time.Hour*96), false, true),
			),
			Entry("Retention period increased while locked",
				generateBackupBucketConfig("bucket", ptr.To(time.Hour*96), true, true),
				generateBackupBucketConfig("bucket", ptr.To(time.Hour*120), true, true),
			),
			Entry("Retention period decreased when not locked",
				generateBackupBucketConfig("bucket", ptr.To(time.Hour*96), false, true),
				generateBackupBucketConfig("bucket", ptr.To(time.Hour*48), false, true),
			),
			Entry("Retention period exactly at minimum (24h)",
				generateBackupBucketConfig("bucket", ptr.To(time.Hour*24), false, true),
				generateBackupBucketConfig("bucket", ptr.To(time.Hour*24), false, true),
			),
			Entry("Transitioning from locked=false to locked=true",
				generateBackupBucketConfig("bucket", ptr.To(time.Hour*96), false, true),
				generateBackupBucketConfig("bucket", ptr.To(time.Hour*96), true, true),
			),
			Entry("Disabling immutability when not locked",
				generateBackupBucketConfig("bucket", ptr.To(time.Hour*96), false, true),
				generateBackupBucketConfig("", nil, false, false),
			),
		)

		DescribeTable("Invalid update scenarios",
			func(oldConfig, newConfig apisgcp.BackupBucketConfig, expectedError string) {
				err := ValidateBackupBucketConfigUpdate(&oldConfig, &newConfig, fldPath).ToAggregate()
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring(expectedError)))
			},
			Entry("Disabling immutable settings is not allowed if locked",
				generateBackupBucketConfig("bucket", ptr.To(time.Hour*96), true, true),
				generateBackupBucketConfig("", nil, false, true),
				"immutability cannot be disabled once it is locked",
			),
			Entry("Unlocking a locked retention policy is not allowed",
				generateBackupBucketConfig("bucket", ptr.To(time.Hour*96), true, true),
				generateBackupBucketConfig("bucket", ptr.To(time.Hour*96), false, true),
				"immutable retention policy lock cannot be unlocked once it is locked",
			),
			Entry("Reducing retention period when locked is not allowed",
				generateBackupBucketConfig("bucket", ptr.To(time.Hour*96), true, true),
				generateBackupBucketConfig("bucket", ptr.To(time.Hour*48), true, true),
				"reducing the retention period from",
			),
		)
	})

	Describe("ValidateBackupBucketCredentialsRef", func() {
		BeforeEach(func() {
			fldPath = field.NewPath("spec", "credentialsRef")
		})

		It("should forbid nil credentialsRef", func() {
			errs := ValidateBackupBucketCredentialsRef(nil, fldPath)
			Expect(errs).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("spec.credentialsRef"),
				"Detail": Equal("must be set"),
			}))))
		})

		It("should forbid v1.ConfigMap credentials", func() {
			credsRef := &corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Name:       "my-creds",
				Namespace:  "my-namespace",
			}
			errs := ValidateBackupBucketCredentialsRef(credsRef, fldPath)
			Expect(errs).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeNotSupported),
				"Field":  Equal("spec.credentialsRef"),
				"Detail": Equal("supported values: \"/v1, Kind=Secret\", \"security.gardener.cloud/v1alpha1, Kind=WorkloadIdentity\""),
			}))))
		})

		It("should allow v1.Secret credentials", func() {
			credsRef := &corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Name:       "my-creds",
				Namespace:  "my-namespace",
			}
			errs := ValidateBackupBucketCredentialsRef(credsRef, fldPath)
			Expect(errs).To(BeEmpty())
		})

		It("should allow security.gardener.cloud/v1alpha1.WorkloadIdentity credentials", func() {
			credsRef := &corev1.ObjectReference{
				APIVersion: "security.gardener.cloud/v1alpha1",
				Kind:       "WorkloadIdentity",
				Name:       "my-creds",
				Namespace:  "my-namespace",
			}
			errs := ValidateBackupBucketCredentialsRef(credsRef, fldPath)
			Expect(errs).To(BeEmpty())
		})
	})
})

func generateBackupBucketConfig(retentionType string, retentionPeriod *time.Duration, locked bool, isImmutabilityConfigured bool) apisgcp.BackupBucketConfig {
	if !isImmutabilityConfigured {
		return apisgcp.BackupBucketConfig{}
	}

	config := apisgcp.BackupBucketConfig{Immutability: &apisgcp.ImmutableConfig{}}

	if retentionType != "" {
		config.Immutability.RetentionType = retentionType
	}

	if retentionPeriod != nil {
		config.Immutability.RetentionPeriod = metav1.Duration{Duration: *retentionPeriod}
		config.Immutability.Locked = locked
	}

	return config
}
