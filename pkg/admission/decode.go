// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package admission

import (
	"github.com/gardener/gardener/extensions/pkg/util"
	"k8s.io/apimachinery/pkg/runtime"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
)

// DecodeWorkerConfig decodes the `WorkerConfig` from the given `RawExtension`.
func DecodeWorkerConfig(decoder runtime.Decoder, worker *runtime.RawExtension) (*apisgcp.WorkerConfig, error) {
	if worker == nil {
		return nil, nil
	}

	workerConfig := &apisgcp.WorkerConfig{}
	if err := util.Decode(decoder, worker.Raw, workerConfig); err != nil {
		return nil, err
	}

	return workerConfig, nil
}

// DecodeControlPlaneConfig decodes the `ControlPlaneConfig` from the given `RawExtension`.
func DecodeControlPlaneConfig(decoder runtime.Decoder, cp *runtime.RawExtension) (*apisgcp.ControlPlaneConfig, error) {
	controlPlaneConfig := &apisgcp.ControlPlaneConfig{}
	if cp == nil {
		return controlPlaneConfig, nil
	}
	if err := util.Decode(decoder, cp.Raw, controlPlaneConfig); err != nil {
		return nil, err
	}

	return controlPlaneConfig, nil
}

// DecodeInfrastructureConfig decodes the `InfrastructureConfig` from the given `RawExtension`.
func DecodeInfrastructureConfig(decoder runtime.Decoder, infra *runtime.RawExtension) (*apisgcp.InfrastructureConfig, error) {
	infraConfig := &apisgcp.InfrastructureConfig{}
	if err := util.Decode(decoder, infra.Raw, infraConfig); err != nil {
		return nil, err
	}

	return infraConfig, nil
}

// DecodeCloudProfileConfig decodes the `CloudProfileConfig` from the given `RawExtension`.
func DecodeCloudProfileConfig(decoder runtime.Decoder, config *runtime.RawExtension) (*apisgcp.CloudProfileConfig, error) {
	cloudProfileConfig := &apisgcp.CloudProfileConfig{}
	if err := util.Decode(decoder, config.Raw, cloudProfileConfig); err != nil {
		return nil, err
	}

	return cloudProfileConfig, nil
}

// DecodeBackupBucketConfig decodes the `BackupBucketConfig` from the given `RawExtension`.
func DecodeBackupBucketConfig(decoder runtime.Decoder, config *runtime.RawExtension) (*apisgcp.BackupBucketConfig, error) {
	backupBucketConfig := &apisgcp.BackupBucketConfig{}

	if config != nil && config.Raw != nil {
		if err := util.Decode(decoder, config.Raw, backupBucketConfig); err != nil {
			return nil, err
		}
	}

	return backupBucketConfig, nil
}
