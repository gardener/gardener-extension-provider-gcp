// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
)

var _ = Describe("ValidateBackupBucketConfig", func() {
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
		Entry("nil config", nil, false, ""),
		Entry("valid config",
			&apisgcp.BackupBucketConfig{
				Immutability: apisgcp.ImmutableConfig{
					RetentionType:   "bucket",
					RetentionPeriod: metav1.Duration{Duration: 24 * time.Hour},
				},
			}, false, ""),
		Entry("missing retentionType",
			&apisgcp.BackupBucketConfig{
				Immutability: apisgcp.ImmutableConfig{
					RetentionType:   "",
					RetentionPeriod: metav1.Duration{Duration: 1 * time.Hour},
				},
			}, true, "must be 'bucket'"),
		Entry("invalid retentionType",
			&apisgcp.BackupBucketConfig{
				Immutability: apisgcp.ImmutableConfig{
					RetentionType:   "invalid",
					RetentionPeriod: metav1.Duration{Duration: 1 * time.Hour},
				},
			}, true, "must be 'bucket'"),
		Entry("non-positive retentionPeriod",
			&apisgcp.BackupBucketConfig{
				Immutability: apisgcp.ImmutableConfig{
					RetentionType:   "bucket",
					RetentionPeriod: metav1.Duration{Duration: 0},
				},
			}, true, "must be a positive duration greater than 24h"),
		Entry("negative retentionPeriod",
			&apisgcp.BackupBucketConfig{
				Immutability: apisgcp.ImmutableConfig{
					RetentionType:   "bucket",
					RetentionPeriod: metav1.Duration{Duration: -1 * time.Hour},
				},
			}, true, "must be a positive duration greater than 24h"),
		Entry("empty retentionPeriod",
			&apisgcp.BackupBucketConfig{
				Immutability: apisgcp.ImmutableConfig{
					RetentionType:   "bucket",
					RetentionPeriod: metav1.Duration{},
				},
			}, true, "must be a positive duration greater than 24h"),
		Entry("retentionPeriod less than 24 hours",
			&apisgcp.BackupBucketConfig{
				Immutability: apisgcp.ImmutableConfig{
					RetentionType:   "bucket",
					RetentionPeriod: metav1.Duration{Duration: 23 * time.Hour},
				},
			}, true, "must be a positive duration greater than 24h"),
	)
})
