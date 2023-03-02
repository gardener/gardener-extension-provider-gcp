// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package admission

import (
	"github.com/gardener/gardener/extensions/pkg/util"
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
