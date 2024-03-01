// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// SetDefaults_MachineImageVersion set the architecture of machine image.
func SetDefaults_MachineImageVersion(obj *MachineImageVersion) {
	if obj.Architecture == nil {
		obj.Architecture = pointer.String(v1beta1constants.ArchitectureAMD64)
	}
}

// SetDefaults_Storage sets the defaults for the managed storage classes
func SetDefaults_Storage(obj *Storage) {
	if obj.ManagedDefaultStorageClass == nil {
		obj.ManagedDefaultStorageClass = pointer.Bool(true)
	}
	if obj.ManagedDefaultVolumeSnapshotClass == nil {
		obj.ManagedDefaultVolumeSnapshotClass = pointer.Bool(true)
	}
}
