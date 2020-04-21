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

package validator

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"

	"k8s.io/apimachinery/pkg/util/sets"

	gcpvalidation "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/validation"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
)

var (
	specPath = field.NewPath("spec")

	networkPath  = specPath.Child("networking")
	providerPath = specPath.Child("provider")

	infrastructureConfigPath = providerPath.Child("infrastructureConfig")
	controlPlaneConfigPath   = providerPath.Child("controlPlaneConfig")
	workersPath              = providerPath.Child("workers")
)

type validationContext struct {
	shoot                *core.Shoot
	infrastructureConfig *gcp.InfrastructureConfig
	controlPlaneConfig   *gcp.ControlPlaneConfig
	cloudProfile         *gardencorev1beta1.CloudProfile
	cloudProfileConfig   *gcp.CloudProfileConfig
}

func workersZones(workers []core.Worker) sets.String {
	var workerZones = sets.NewString()
	for _, worker := range workers {
		workerZones.Insert(worker.Zones...)
	}
	return workerZones
}

// getAllowedRegionZonesFromCloudProfile fetches the set of allowed zones from the Cloud Profile.
func getAllowedRegionZonesFromCloudProfile(shoot *core.Shoot, cloudProfile *gardencorev1beta1.CloudProfile) sets.String {
	shootRegion := shoot.Spec.Region
	for _, region := range cloudProfile.Spec.Regions {
		if region.Name == shootRegion {
			gcpZones := sets.NewString()
			for _, gcpZone := range region.Zones {
				gcpZones.Insert(gcpZone.Name)
			}
			return gcpZones
		}
	}
	return nil
}

func (v *Shoot) validateShoot(valContext *validationContext) field.ErrorList {
	var (
		allErrors    = field.ErrorList{}
		allowedZones = getAllowedRegionZonesFromCloudProfile(valContext.shoot, valContext.cloudProfile)
	)

	allErrors = append(allErrors, gcpvalidation.ValidateNetworking(valContext.shoot.Spec.Networking, networkPath)...)
	allErrors = append(allErrors, gcpvalidation.ValidateInfrastructureConfig(valContext.infrastructureConfig, valContext.shoot.Spec.Networking.Nodes, valContext.shoot.Spec.Networking.Pods, valContext.shoot.Spec.Networking.Services, infrastructureConfigPath)...)
	allErrors = append(allErrors, gcpvalidation.ValidateWorkers(valContext.shoot.Spec.Provider.Workers, workersPath)...)
	allErrors = append(allErrors, gcpvalidation.ValidateControlPlaneConfig(valContext.controlPlaneConfig, allowedZones, workersZones(valContext.shoot.Spec.Provider.Workers), controlPlaneConfigPath)...)

	// WorkerConfig
	for i, worker := range valContext.shoot.Spec.Provider.Workers {
		workerFldPath := workersPath.Index(i)
		for _, volume := range worker.DataVolumes {
			workerConfig, err := decodeWorkerConfig(v.decoder, worker.ProviderConfig)
			if err != nil {
				allErrors = append(allErrors, field.Invalid(workerFldPath.Child("providerConfig"), err, "invalid providerConfig"))
			} else {
				allErrors = append(allErrors, gcpvalidation.ValidateWorkerConfig(workerConfig, volume.Type)...)
			}
		}
	}

	return allErrors
}

func (v *Shoot) validateShootCreate(ctx context.Context, shoot *core.Shoot) error {
	validationContext, err := newValidationContext(ctx, v.decoder, v.client, shoot)
	if err != nil {
		return err
	}

	return v.validateShoot(validationContext).ToAggregate()
}

func (v *Shoot) validateShootUpdate(ctx context.Context, oldShoot, currentShoot *core.Shoot) error {
	oldValContext, err := newValidationContext(ctx, v.decoder, v.client, oldShoot)
	if err != nil {
		return err
	}

	currentValContext, err := newValidationContext(ctx, v.decoder, v.client, currentShoot)
	if err != nil {
		return err
	}

	var (
		oldInfrastructureConfig, currentInfrastructureConfig = oldValContext.infrastructureConfig, currentValContext.infrastructureConfig
		oldControlPlaneConfig, currentControlPlaneConfig     = oldValContext.controlPlaneConfig, currentValContext.controlPlaneConfig
		allErrors                                            = field.ErrorList{}
	)

	if !reflect.DeepEqual(oldInfrastructureConfig, currentInfrastructureConfig) {
		allErrors = append(allErrors, gcpvalidation.ValidateInfrastructureConfigUpdate(oldInfrastructureConfig, currentInfrastructureConfig, infrastructureConfigPath)...)
	}

	if !reflect.DeepEqual(oldControlPlaneConfig, currentControlPlaneConfig) {
		allErrors = append(allErrors, gcpvalidation.ValidateControlPlaneConfigUpdate(oldControlPlaneConfig, currentControlPlaneConfig, controlPlaneConfigPath)...)
	}

	allErrors = append(allErrors, gcpvalidation.ValidateWorkersUpdate(oldValContext.shoot.Spec.Provider.Workers, currentValContext.shoot.Spec.Provider.Workers, workersPath)...)
	allErrors = append(allErrors, v.validateShoot(currentValContext)...)

	return allErrors.ToAggregate()

}

func newValidationContext(ctx context.Context, decoder runtime.Decoder, c client.Client, shoot *core.Shoot) (*validationContext, error) {
	if shoot.Spec.Provider.InfrastructureConfig == nil {
		return nil, field.Required(infrastructureConfigPath, "infrastructureConfig must be set for GCP shoots")
	}
	infrastructureConfig, err := decodeInfrastructureConfig(decoder, shoot.Spec.Provider.InfrastructureConfig)
	if err != nil {
		return nil, fmt.Errorf("error decoding infrastructureConfig: %v", err)
	}

	if shoot.Spec.Provider.ControlPlaneConfig == nil {
		return nil, field.Required(controlPlaneConfigPath, "controlPlaneConfig must be set for GCP shoots")
	}
	controlPlaneConfig, err := decodeControlPlaneConfig(decoder, shoot.Spec.Provider.ControlPlaneConfig)
	if err != nil {
		return nil, fmt.Errorf("error decoding controlPlaneConfig: %v", err)
	}

	cloudProfile := &gardencorev1beta1.CloudProfile{}
	if err := c.Get(ctx, kutil.Key(shoot.Spec.CloudProfileName), cloudProfile); err != nil {
		return nil, err
	}

	if cloudProfile.Spec.ProviderConfig == nil {
		return nil, fmt.Errorf("providerConfig is not given for cloud profile %q", cloudProfile.Name)
	}
	cloudProfileConfig, err := decodeCloudProfileConfig(decoder, cloudProfile.Spec.ProviderConfig)
	if err != nil {
		return nil, fmt.Errorf("an error occurred while reading the cloud profile %q: %v", cloudProfile.Name, err)
	}

	return &validationContext{
		shoot:                shoot,
		infrastructureConfig: infrastructureConfig,
		controlPlaneConfig:   controlPlaneConfig,
		cloudProfile:         cloudProfile,
		cloudProfileConfig:   cloudProfileConfig,
	}, nil
}
