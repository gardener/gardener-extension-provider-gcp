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

package network

import (
	calicov1alpha1helper "github.com/gardener/gardener-extension-networking-calico/pkg/apis/calico/v1alpha1/helper"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"

	calicov1alpha1 "github.com/gardener/gardener-extension-networking-calico/pkg/apis/calico/v1alpha1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func mutateNetworkConfig(new, old *extensionsv1alpha1.Network) error {
	extensionswebhook.LogMutation(logger, "Network", new.Namespace, new.Name)

	var (
		networkConfig *calicov1alpha1.NetworkConfig
		backendNone   = calicov1alpha1.None
		err           error
	)

	if new.Spec.ProviderConfig != nil {
		networkConfig, err = calicov1alpha1helper.CalicoNetworkConfigFromNetworkResource(new)
		if err != nil {
			return err
		}
	} else {
		networkConfig = &calicov1alpha1.NetworkConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: calicov1alpha1.SchemeGroupVersion.String(),
				Kind:       "NetworkConfig",
			},
		}
	}

	networkConfig.Backend = &backendNone
	new.Spec.ProviderConfig = &runtime.RawExtension{
		Object: networkConfig,
	}

	return nil
}
