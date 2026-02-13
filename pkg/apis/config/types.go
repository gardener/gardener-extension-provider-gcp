// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	apisconfigv1alpha1 "github.com/gardener/gardener/extensions/pkg/apis/config/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControllerConfiguration defines the configuration for the GCP provider.
type ControllerConfiguration struct {
	metav1.TypeMeta

	// ClientConnection specifies the kubeconfig file and client connection
	// settings for the proxy server to use when communicating with the apiserver.
	ClientConnection *componentbaseconfigv1alpha1.ClientConnectionConfiguration
	// ETCD is the etcd configuration.
	ETCD ETCD
	// HealthCheckConfig is the config for the health check controller
	HealthCheckConfig *apisconfigv1alpha1.HealthCheckConfig
	// FeatureGates is a map of feature names to bools that enable
	// or disable alpha/experimental features.
	// Default: nil
	FeatureGates map[string]bool

	// Profiling holds configuration for profiling and debugging related features.
	// This configuration is meant for debugging purposes only
	// and should be used in production with caution
	// as pprof can expose sensitive information and impact performance.
	Profiling *ProfilingConfiguration
}

// ProfilingConfiguration contains debugging and profiling configuration.
type ProfilingConfiguration struct {
	// PprofBindAddress is the TCP address that the controller should bind to for serving pprof.
	PprofBindAddress *string
	// EnableContentionProfiling enables block profiling, if PprofBindAddress is set.
	EnableContentionProfiling *bool
}

// ETCD is an etcd configuration.
type ETCD struct {
	// ETCDStorage is the etcd storage configuration.
	Storage ETCDStorage
	// ETCDBackup is the etcd backup configuration.
	Backup ETCDBackup
}

// ETCDStorage is an etcd storage configuration.
type ETCDStorage struct {
	// ClassName is the name of the storage class used in etcd-main volume claims.
	ClassName *string
	// Capacity is the storage capacity used in etcd-main volume claims.
	Capacity *resource.Quantity
}

// ETCDBackup is an etcd backup configuration.
type ETCDBackup struct {
	// Schedule is the etcd backup schedule.
	Schedule *string
}

// WorkloadIdentity is a configuration that specifies how workload identity configs are validated.
type WorkloadIdentity struct {
	// AllowedTokenURLs are the allowed token URLs.
	AllowedTokenURLs []string
	// AllowedServiceAccountImpersonationURLRegExps are the allowed service account impersonation URL regular expressions.
	AllowedServiceAccountImpersonationURLRegExps []string
}

// BackupBucket is a configuration that specifies how backupbucket configs should be validated.
type BackupBucket struct {
	// AllowedEndpointOverrideURLs are the allowed endpointOverride URLs.
	AllowedEndpointOverrideURLs []string
}
