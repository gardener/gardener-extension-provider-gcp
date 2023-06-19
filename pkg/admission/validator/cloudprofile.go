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

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/admission"
	gcpvalidation "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/validation"
)

type cloudProfile struct {
	decoder runtime.Decoder
}

// NewCloudProfileValidator returns a new instance of a cloud profile validator.
func NewCloudProfileValidator() extensionswebhook.Validator {
	return &cloudProfile{}
}

// InjectScheme injects the given scheme into the validator.
func (cp *cloudProfile) InjectScheme(scheme *runtime.Scheme) error {
	cp.decoder = serializer.NewCodecFactory(scheme, serializer.EnableStrict).UniversalDecoder()
	return nil
}

var cpProviderConfigPath = specPath.Child("providerConfig")

// Validate validates the given cloud profile objects.
func (cp *cloudProfile) Validate(_ context.Context, new, _ client.Object) error {
	cloudProfile, ok := new.(*core.CloudProfile)
	if !ok {
		return fmt.Errorf("wrong object type %T", new)
	}

	if cloudProfile.Spec.ProviderConfig == nil {
		return field.Required(cpProviderConfigPath, "providerConfig must be set for GCP cloud profiles")
	}

	cpConfig, err := admission.DecodeCloudProfileConfig(cp.decoder, cloudProfile.Spec.ProviderConfig)
	if err != nil {
		return err
	}

	return gcpvalidation.ValidateCloudProfileConfig(cpConfig, cloudProfile.Spec.MachineImages, cpProviderConfigPath).ToAggregate()
}
