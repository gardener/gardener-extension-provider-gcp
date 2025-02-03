// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"fmt"
	"reflect"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils/gardener"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/admission"
	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	gcpvalidation "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/validation"
)

type shoot struct {
	client         client.Client
	decoder        runtime.Decoder
	lenientDecoder runtime.Decoder
}

// NewShootValidator returns a new instance of a shoot validator.
func NewShootValidator(mgr manager.Manager) extensionswebhook.Validator {
	return &shoot{
		client:         mgr.GetClient(),
		decoder:        serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
		lenientDecoder: serializer.NewCodecFactory(mgr.GetScheme()).UniversalDecoder(),
	}
}

// Validate validates the given shoot objects.
func (s *shoot) Validate(ctx context.Context, newObj, oldObj client.Object) error {
	shoot, ok := newObj.(*core.Shoot)
	if !ok {
		return fmt.Errorf("wrong object type %T", newObj)
	}

	// Skip if it's a workerless Shoot
	if gardencorehelper.IsWorkerless(shoot) {
		return nil
	}

	if oldObj != nil {
		oldShoot, ok := oldObj.(*core.Shoot)
		if !ok {
			return fmt.Errorf("wrong object type %T for old object", oldObj)
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
	cloudProfileSpec     *gardencorev1beta1.CloudProfileSpec
	cloudProfileConfig   *apisgcp.CloudProfileConfig
}

func workersZones(workers []core.Worker) sets.Set[string] {
	workerZones := sets.New[string]()
	for _, worker := range workers {
		workerZones.Insert(worker.Zones...)
	}
	return workerZones
}

// getAllowedRegionZonesFromCloudProfile fetches the set of allowed zones from the Cloud Profile.
func getAllowedRegionZonesFromCloudProfile(shoot *core.Shoot, cloudProfileSpec *gardencorev1beta1.CloudProfileSpec) sets.Set[string] {
	shootRegion := shoot.Spec.Region
	for _, region := range cloudProfileSpec.Regions {
		if region.Name == shootRegion {
			gcpZones := sets.New[string]()
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
		allowedZones = getAllowedRegionZonesFromCloudProfile(valContext.shoot, valContext.cloudProfileSpec)
	)

	if valContext.shoot.Spec.Networking != nil {
		allErrors = append(allErrors, gcpvalidation.ValidateNetworking(valContext.shoot.Spec.Networking, networkPath)...)
		allErrors = append(allErrors, gcpvalidation.ValidateInfrastructureConfig(valContext.infrastructureConfig, valContext.shoot.Spec.Networking.Nodes, valContext.shoot.Spec.Networking.Pods, valContext.shoot.Spec.Networking.Services, infrastructureConfigPath)...)
	}

	allErrors = append(allErrors, gcpvalidation.ValidateWorkers(valContext.shoot.Spec.Provider.Workers, workersPath)...)
	allErrors = append(allErrors, gcpvalidation.ValidateControlPlaneConfig(valContext.controlPlaneConfig, allowedZones, workersZones(valContext.shoot.Spec.Provider.Workers), valContext.shoot.Spec.Kubernetes.Version, controlPlaneConfigPath)...)

	// WorkerConfig
	for i, worker := range valContext.shoot.Spec.Provider.Workers {
		workerFldPath := workersPath.Index(i)
		workerConfig, err := admission.DecodeWorkerConfig(s.decoder, worker.ProviderConfig)
		if err != nil {
			allErrors = append(allErrors, field.Invalid(workerFldPath.Child("providerConfig"), err, "invalid providerConfig"))
		} else {
			allErrors = append(allErrors, gcpvalidation.ValidateWorkerConfig(workerConfig, worker.DataVolumes)...)
		}
	}

	return allErrors
}

func (s *shoot) validateCreate(ctx context.Context, shoot *core.Shoot) error {
	validationContext, err := newValidationContext(ctx, s.decoder, s.client, shoot)
	if err != nil {
		return err
	}

	return s.validateContext(validationContext).ToAggregate()
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

	shootV1Beta1 := &gardencorev1beta1.Shoot{}
	err = gardencorev1beta1.Convert_core_Shoot_To_v1beta1_Shoot(shoot, shootV1Beta1, nil)
	if err != nil {
		return nil, err
	}
	cloudProfile, err := gardener.GetCloudProfile(ctx, c, shootV1Beta1)
	if err != nil {
		return nil, err
	}
	if cloudProfile == nil {
		return nil, fmt.Errorf("cloudprofile could not be found")
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
		cloudProfileSpec:     &cloudProfile.Spec,
		cloudProfileConfig:   cloudProfileConfig,
	}, nil
}
