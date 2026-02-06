// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"fmt"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/admission"
	gcpvalidation "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/validation"
)

type cloudProfileValidator struct {
	decoder runtime.Decoder
}

// NewCloudProfileValidator returns a new instance of a cloud profile validator.
func NewCloudProfileValidator(mgr manager.Manager) extensionswebhook.Validator {
	return &cloudProfileValidator{
		decoder: serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
	}
}

// Validate validates the given cloud profile objects.
func (cp *cloudProfileValidator) Validate(_ context.Context, newObj, _ client.Object) error {
	cloudProfile, ok := newObj.(*core.CloudProfile)
	if !ok {
		return fmt.Errorf("wrong object type %T", newObj)
	}

	if cloudProfile.Spec.ProviderConfig == nil {
		return field.Required(specPath.Child("providerConfig"), "providerConfig must be set for GCP cloud profiles")
	}

	cpConfig, err := admission.DecodeCloudProfileConfig(cp.decoder, cloudProfile.Spec.ProviderConfig)
	if err != nil {
		return fmt.Errorf("could not decode providerConfig of CloudProfile %q: %w", cloudProfile.Name, err)
	}

	capabilityDefinitions, err := gardencorev1beta1helper.ConvertV1beta1CapabilityDefinitions(cloudProfile.Spec.MachineCapabilities)
	if err != nil {
		return field.InternalError(specPath.Child("machineCapabilities"), err)
	}

	return gcpvalidation.ValidateCloudProfileConfig(cpConfig, cloudProfile.Spec.MachineImages, capabilityDefinitions, specPath).ToAggregate()
}
