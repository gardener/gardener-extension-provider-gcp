// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package admission

import (
	"github.com/gardener/gardener/extensions/pkg/util"
	"github.com/gardener/gardener/pkg/apis/core"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
)

// DecodeWorkerConfig decodes the `WorkerConfig` from the given `RawExtension`.
func DecodeWorkerConfig(decoder runtime.Decoder, worker *runtime.RawExtension) (*gcp.WorkerConfig, error) {
	if worker == nil {
		return nil, nil
	}

	workerConfig := &gcp.WorkerConfig{}
	if err := util.Decode(decoder, worker.Raw, workerConfig); err != nil {
		return nil, err
	}

	return workerConfig, nil
}

// DecodeControlPlaneConfig decodes the `ControlPlaneConfig` from the given `RawExtension`.
func DecodeControlPlaneConfig(decoder runtime.Decoder, cp *runtime.RawExtension) (*gcp.ControlPlaneConfig, error) {
	controlPlaneConfig := &gcp.ControlPlaneConfig{}
	if err := util.Decode(decoder, cp.Raw, controlPlaneConfig); err != nil {
		return nil, err
	}

	return controlPlaneConfig, nil
}

// DecodeInfrastructureConfig decodes the `InfrastructureConfig` from the given `RawExtension`.
func DecodeInfrastructureConfig(decoder runtime.Decoder, infra *runtime.RawExtension) (*gcp.InfrastructureConfig, error) {
	infraConfig := &gcp.InfrastructureConfig{}
	if err := util.Decode(decoder, infra.Raw, infraConfig); err != nil {
		return nil, err
	}

	return infraConfig, nil
}

// DecodeCloudProfileConfig decodes the `CloudProfileConfig` from the given `RawExtension`.
func DecodeCloudProfileConfig(decoder runtime.Decoder, config *runtime.RawExtension) (*gcp.CloudProfileConfig, error) {
	cloudProfileConfig := &gcp.CloudProfileConfig{}
	if err := util.Decode(decoder, config.Raw, cloudProfileConfig); err != nil {
		return nil, err
	}

	return cloudProfileConfig, nil
}

// DecodeBackupBucketConfig decodes the `BackupBucketConfig` from the given `RawExtension`.
func DecodeBackupBucketConfig(decoder runtime.Decoder, config *runtime.RawExtension) (*gcp.BackupBucketConfig, error) {
	if config == nil {
		return nil, nil
	}

	backupBucketConfig := &gcp.BackupBucketConfig{}
	if err := util.Decode(decoder, config.Raw, backupBucketConfig); err != nil {
		return nil, err
	}

	return backupBucketConfig, nil
}

// DecodeSeedBackupBucketConfig decodes the `BackupBucketConfig` from the given `SeedBackup`.
func DecodeSeedBackupBucketConfig(decoder runtime.Decoder, backup *core.SeedBackup) (*gcp.BackupBucketConfig, error) {
	if backup == nil || backup.ProviderConfig == nil {
		return nil, nil
	}

	return DecodeBackupBucketConfig(decoder, backup.ProviderConfig)
}
