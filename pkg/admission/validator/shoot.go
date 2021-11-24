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

	"github.com/gardener/gardener-extension-provider-gcp/pkg/admission"
	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	gcpvalidation "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/validation"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type shoot struct {
	client         client.Client
	apiReader      client.Reader
	decoder        runtime.Decoder
	lenientDecoder runtime.Decoder
}

// NewShootValidator returns a new instance of a shoot validator.
func NewShootValidator() extensionswebhook.Validator {
	return &shoot{}
}

// InjectScheme injects the given scheme into the validator.
func (s *shoot) InjectScheme(scheme *runtime.Scheme) error {
	s.decoder = serializer.NewCodecFactory(scheme, serializer.EnableStrict).UniversalDecoder()
	s.lenientDecoder = serializer.NewCodecFactory(scheme).UniversalDecoder()
	return nil
}

// InjectClient injects the given client into the validator.
func (s *shoot) InjectClient(client client.Client) error {
	s.client = client
	return nil
}

// InjectAPIReader injects the given apiReader into the validator.
func (s *shoot) InjectAPIReader(apiReader client.Reader) error {
	s.apiReader = apiReader
	return nil
}

// Validate validates the given shoot objects.
func (s *shoot) Validate(ctx context.Context, new, old client.Object) error {
	shoot, ok := new.(*core.Shoot)
	if !ok {
		return fmt.Errorf("wrong object type %T", new)
	}

	if old != nil {
		oldShoot, ok := old.(*core.Shoot)
		if !ok {
			return fmt.Errorf("wrong object type %T for old object", old)
		}
		return s.validateUpdate(ctx, oldShoot, shoot)
	}

	return s.validateCreate(ctx, shoot)
}

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
	infrastructureConfig *apisgcp.InfrastructureConfig
	controlPlaneConfig   *apisgcp.ControlPlaneConfig
	cloudProfile         *gardencorev1beta1.CloudProfile
	cloudProfileConfig   *apisgcp.CloudProfileConfig
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

func (s *shoot) validateContext(valContext *validationContext) field.ErrorList {
	var (
		allErrors    = field.ErrorList{}
		allowedZones = getAllowedRegionZonesFromCloudProfile(valContext.shoot, valContext.cloudProfile)
	)

	allErrors = append(allErrors, gcpvalidation.ValidateNetworking(valContext.shoot.Spec.Networking, networkPath)...)
	allErrors = append(allErrors, gcpvalidation.ValidateInfrastructureConfig(valContext.infrastructureConfig, valContext.shoot.Spec.Networking.Nodes, valContext.shoot.Spec.Networking.Pods, valContext.shoot.Spec.Networking.Services, infrastructureConfigPath)...)
	allErrors = append(allErrors, gcpvalidation.ValidateWorkers(valContext.shoot.Spec.Provider.Workers, workersPath)...)
	allErrors = append(allErrors, gcpvalidation.ValidateControlPlaneConfig(valContext.controlPlaneConfig, allowedZones, workersZones(valContext.shoot.Spec.Provider.Workers), valContext.shoot.Spec.Kubernetes.Version, controlPlaneConfigPath)...)

	// WorkerConfig
	for i, worker := range valContext.shoot.Spec.Provider.Workers {
		workerFldPath := workersPath.Index(i)
		for _, volume := range worker.DataVolumes {
			workerConfig, err := admission.DecodeWorkerConfig(s.decoder, worker.ProviderConfig)
			if err != nil {
				allErrors = append(allErrors, field.Invalid(workerFldPath.Child("providerConfig"), err, "invalid providerConfig"))
			} else {
				allErrors = append(allErrors, gcpvalidation.ValidateWorkerConfig(workerConfig, volume.Type)...)
			}
		}
	}

	return allErrors
}

func (s *shoot) validateCreate(ctx context.Context, shoot *core.Shoot) error {
	validationContext, err := newValidationContext(ctx, s.decoder, s.client, shoot)
	if err != nil {
		return err
	}

	// TODO: This check won't be needed after generic support to scale from zero is introduced in CA
	// Ongoing issue - https://github.com/gardener/autoscaler/issues/27
	for i, worker := range shoot.Spec.Provider.Workers {
		if err = gcpvalidation.ValidateWorkerAutoScaling(worker, workersPath.Index(i).Child("minimum").String()); err != nil {
			return err
		}
	}

	if err := s.validateContext(validationContext).ToAggregate(); err != nil {
		return err
	}

	return s.validateShootSecret(ctx, shoot)
}

func (s *shoot) validateUpdate(ctx context.Context, oldShoot, currentShoot *core.Shoot) error {
	oldValContext, err := newValidationContext(ctx, s.lenientDecoder, s.client, oldShoot)
	if err != nil {
		return err
	}

	currentValContext, err := newValidationContext(ctx, s.decoder, s.client, currentShoot)
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
	allErrors = append(allErrors, s.validateContext(currentValContext)...)

	return allErrors.ToAggregate()

}

func newValidationContext(ctx context.Context, decoder runtime.Decoder, c client.Client, shoot *core.Shoot) (*validationContext, error) {
	if shoot.Spec.Provider.InfrastructureConfig == nil {
		return nil, field.Required(infrastructureConfigPath, "infrastructureConfig must be set for GCP shoots")
	}
	infrastructureConfig, err := admission.DecodeInfrastructureConfig(decoder, shoot.Spec.Provider.InfrastructureConfig)
	if err != nil {
		return nil, fmt.Errorf("error decoding infrastructureConfig: %v", err)
	}

	if shoot.Spec.Provider.ControlPlaneConfig == nil {
		return nil, field.Required(controlPlaneConfigPath, "controlPlaneConfig must be set for GCP shoots")
	}
	controlPlaneConfig, err := admission.DecodeControlPlaneConfig(decoder, shoot.Spec.Provider.ControlPlaneConfig)
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
	cloudProfileConfig, err := admission.DecodeCloudProfileConfig(decoder, cloudProfile.Spec.ProviderConfig)
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

func (s *shoot) validateShootSecret(ctx context.Context, shoot *core.Shoot) error {
	var (
		secretBinding    = &gardencorev1beta1.SecretBinding{}
		secretBindingKey = kutil.Key(shoot.Namespace, shoot.Spec.SecretBindingName)
	)
	if err := kutil.LookupObject(ctx, s.client, s.apiReader, secretBindingKey, secretBinding); err != nil {
		return err
	}

	var (
		secret    = &corev1.Secret{}
		secretKey = kutil.Key(secretBinding.SecretRef.Namespace, secretBinding.SecretRef.Name)
	)
	// Explicitly use the client.Reader to prevent controller-runtime to start Informer for Secrets
	// under the hood. The latter increases the memory usage of the component.
	if err := s.apiReader.Get(ctx, secretKey, secret); err != nil {
		return err
	}

	return gcpvalidation.ValidateCloudProviderSecret(secret)
}
