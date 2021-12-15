// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package cloudprovider

import (
	"context"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ensurer struct {
	logger logr.Logger
	client client.Client
}

// NewEnsurer ...
func NewEnsurer(logger logr.Logger) cloudprovider.Ensurer {
	return &ensurer{
		logger: logger,
	}
}

// InjectClient injects the given client into the ensurer.
func (e *ensurer) InjectClient(client client.Client) error {
	e.client = client
	return nil
}

// InjectScheme injects the given scheme into the decoder of the ensurer.
func (e *ensurer) InjectScheme(_ *runtime.Scheme) error {
	return nil
}

// EnsureCloudProviderSecret ...
func (e *ensurer) EnsureCloudProviderSecret(ctx context.Context, _ gcontext.GardenContext, new, old *corev1.Secret) error {
	return nil
}
