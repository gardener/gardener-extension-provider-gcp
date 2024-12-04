// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"time"

	"k8s.io/apimachinery/pkg/util/validation/field"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
)

// ValidateBackupBucketConfig validates a BackupBucketConfig object.
func ValidateBackupBucketConfig(config *apisgcp.BackupBucketConfig, fldPath *field.Path) field.ErrorList {
	if config == nil {
		return nil
	}
	allErrs := field.ErrorList{}

	// Currently, only 'bucket' type is supported. In the future, 'object' type will be supported.
	if config.Immutability.RetentionType != "bucket" {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("immutability", "retentionType"), config.Immutability.RetentionType, "must be 'bucket'"))
	}

	// The minimum retention period is 24 hours as per Google Cloud Storage requirements.
	// Reference: https://github.com/googleapis/google-cloud-go/blob/3005f5a86c18254e569b8b1782bf014aa62f33cc/storage/bucket.go#L1430-L1434
	if config.Immutability.RetentionPeriod.Duration < 24*time.Hour {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("immutability", "retentionPeriod"), config.Immutability.RetentionPeriod.Duration.String(), "must be a positive duration greater than 24h"))
	}

	return allErrs
}
