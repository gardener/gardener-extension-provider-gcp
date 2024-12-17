// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackupBucketConfig represents the configuration for a backup bucket.
type BackupBucketConfig struct {
	metav1.TypeMeta

	// Immutability defines the immutability config for the backup bucket.
	Immutability *ImmutableConfig
}

// ImmutableConfig represents the immutability configuration for a backup bucket.
type ImmutableConfig struct {
	// RetentionType specifies the type of retention for the backup bucket.
	// Currently allowed values are:
	// - "bucket": The retention policy applies to the entire bucket.
	RetentionType string

	// RetentionPeriod specifies the immutability retention period for the backup bucket.
	// The minimum retention period is 24 hours as per Google Cloud Storage requirements.
	// Reference: https://github.com/googleapis/google-cloud-go/blob/3005f5a86c18254e569b8b1782bf014aa62f33cc/storage/bucket.go#L1430-L1434
	RetentionPeriod metav1.Duration

	// Locked indicates whether the immutable retention policy is locked for the backup bucket.
	// If set to true, the retention policy cannot be removed or the retention period reduced, enforcing immutability.
	Locked bool
}
