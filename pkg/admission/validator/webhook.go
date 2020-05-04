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
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"

	extensionspredicate "github.com/gardener/gardener/extensions/pkg/predicate"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var logger = log.Log.WithName("gcp-validator-webhook")

// New creates a new validation webhook for `core.gardener.cloud` resources.
func New(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	logger.Info("Setting up webhook")

	return extensionswebhook.New(mgr, extensionswebhook.Args{
		Provider:   gcp.Type,
		Name:       extensionswebhook.ValidatorName,
		Path:       extensionswebhook.ValidatorPath,
		Predicates: []predicate.Predicate{extensionspredicate.GardenCoreProviderType(gcp.Type)},
		Validators: map[extensionswebhook.Validator][]runtime.Object{
			NewShootValidator():        {&core.Shoot{}},
			NewCloudProfileValidator(): {&core.CloudProfile{}},
		},
	})
}
