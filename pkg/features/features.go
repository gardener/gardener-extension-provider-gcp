// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package features

import (
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/component-base/featuregate"
)

const (
	// DisableGardenerServiceAccountCreation controls whether the gcp provider will create a default service account for VMs managed by MCM.
	// beta: v1.29.0
	DisableGardenerServiceAccountCreation featuregate.Feature = "DisableGardenerServiceAccountCreation"
)

// ExtensionFeatureGate is the feature gate for the extension controllers.
var ExtensionFeatureGate = featuregate.NewFeatureGate()

// RegisterExtensionFeatureGate registers features to the extension feature gate.
func RegisterExtensionFeatureGate() {
	runtime.Must(ExtensionFeatureGate.Add(map[featuregate.Feature]featuregate.FeatureSpec{
		DisableGardenerServiceAccountCreation: {Default: true, PreRelease: featuregate.Beta},
	}))
}
