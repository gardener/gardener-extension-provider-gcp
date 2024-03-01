// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
)

var _ = Describe("Defaults", func() {
	Describe("#SetDefaults_MachineImageVersion", func() {
		It("should default the architecture to amd64", func() {
			obj := &MachineImageVersion{}

			SetDefaults_MachineImageVersion(obj)

			Expect(*obj.Architecture).To(Equal(v1beta1constants.ArchitectureAMD64))
		})
	})
	Describe("#SetDefaults_Storage", func() {
		It("should default to managed storage classes", func() {
			obj := &Storage{}

			SetDefaults_Storage(obj)

			Expect(*obj.ManagedDefaultStorageClass).To(Equal(true))
			Expect(*obj.ManagedDefaultVolumeSnapshotClass).To(Equal(true))
		})
	})
})
