// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure_test

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/features"
)

var (
	featureGates = map[string]bool{
		string(features.DisableGardenerServiceAccountCreation): true,
	}
)

func TestInfrastructure(t *testing.T) {
	RegisterFailHandler(Fail)
	features.RegisterExtensionFeatureGate()

	err := features.ExtensionFeatureGate.SetFromMap(featureGates)
	if err != nil {
		Fail(fmt.Sprintf("failed to register feature gates: %v", err))
	}

	RunSpecs(t, "Infrastructure Suite")
}
