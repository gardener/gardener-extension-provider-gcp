// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkloadIdentityConfig contains configuration settings for workload identity.
type WorkloadIdentityConfig struct {
	metav1.TypeMeta

	// ProjectID is the ID of the GCP project.
	ProjectID string
	// CredentialsConfig contains information for workload authentication against GCP.
	CredentialsConfig *runtime.RawExtension
}
