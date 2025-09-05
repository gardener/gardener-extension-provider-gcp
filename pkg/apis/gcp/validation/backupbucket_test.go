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

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
)

var _ = Describe("BackupBucket", func() {
	Describe("ValidateBackupBucketConfig", func() {
		var fldPath *field.Path

		BeforeEach(func() {
			fldPath = field.NewPath("spec")
		})

		DescribeTable("validation cases",
			func(config *apisgcp.BackupBucketConfig, wantErr bool, errMsg string) {
				errs := ValidateBackupBucketConfig(config, fldPath)
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
		)
	})

	Describe("ValidateBackupBucketCredentialsRef", func() {
		var fldPath *field.Path

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
