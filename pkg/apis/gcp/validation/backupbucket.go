// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"time"

	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
)

// ValidateBackupBucketConfig validates a BackupBucketConfig object.
func ValidateBackupBucketConfig(config *apisgcp.BackupBucketConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if config != nil && config.Immutability != nil {
		// Currently, only 'bucket' type is supported.
		if config.Immutability.RetentionType != "bucket" {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("immutability", "retentionType"), config.Immutability.RetentionType, "must be 'bucket'"))
		}

		// The minimum retention period is 24 hours as per Google Cloud Storage requirements.
		// Reference: https://github.com/googleapis/google-cloud-go/blob/3005f5a86c18254e569b8b1782bf014aa62f33cc/storage/bucket.go#L1430-L1434
		if config.Immutability.RetentionPeriod.Duration < 24*time.Hour {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("immutability", "retentionPeriod"), config.Immutability.RetentionPeriod.Duration.String(), "must be a positive duration greater than 24h"))
		}
	}

	return allErrs
}

// ValidateBackupBucketCredentialsRef validates credentialsRef is set to supported kind of credentials.
func ValidateBackupBucketCredentialsRef(credentialsRef *corev1.ObjectReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if credentialsRef == nil {
		return append(allErrs, field.Required(fldPath, "must be set"))
	}

	var (
		secretGVK           = corev1.SchemeGroupVersion.WithKind("Secret")
		workloadIdentityGVK = securityv1alpha1.SchemeGroupVersion.WithKind("WorkloadIdentity")

		allowedGVKs = sets.New(secretGVK, workloadIdentityGVK)
		validGVKs   = []string{secretGVK.String(), workloadIdentityGVK.String()}
	)

	if !allowedGVKs.Has(credentialsRef.GroupVersionKind()) {
		allErrs = append(allErrs, field.NotSupported(fldPath, credentialsRef.String(), validGVKs))
	}

	return allErrs
}
