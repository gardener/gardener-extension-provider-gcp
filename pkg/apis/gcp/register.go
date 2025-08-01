// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupName is the group name use in this package
const GroupName = "gcp.provider.extensions.gardener.cloud"

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: runtime.APIVersionInternal}

// Kind takes an unqualified kind and returns a Group qualified GroupKind
func Kind(kind string) schema.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

var (
	// schemeBuilder used to register the Shoot resource.
	schemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	// AddToScheme is a pointer to schemeBuilder.AddToScheme.
	AddToScheme = schemeBuilder.AddToScheme
)

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&CloudProfileConfig{},
		&InfrastructureConfig{},
		&InfrastructureStatus{},
		&InfrastructureState{},
		&ControlPlaneConfig{},
		&WorkerStatus{},
		&WorkerConfig{},
		&BackupBucketConfig{},
		&WorkloadIdentityConfig{},
	)
	return nil
}
