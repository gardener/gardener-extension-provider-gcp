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

package validation

import (
	"fmt"
	"regexp"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/internal"
	corev1 "k8s.io/api/core/v1"
)

var projectIDRegexp = regexp.MustCompile(`^(?P<project>[a-z][a-z0-9-]{4,28}[a-z0-9])$`)

// ValidateCloudProviderSecret checks whether the given secret contains a valid GCP service account.
func ValidateCloudProviderSecret(secret *corev1.Secret) error {
	serviceAccountJSON, ok := secret.Data[gcp.ServiceAccountJSONField]
	if !ok {
		return fmt.Errorf("missing %q field in secret", gcp.ServiceAccountJSONField)
	}

	projectID, err := internal.ExtractServiceAccountProjectID(serviceAccountJSON)
	if err != nil {
		return err
	}

	if !projectIDRegexp.MatchString(projectID) {
		return fmt.Errorf("service account project ID does not match the expected format '%s'", projectIDRegexp)
	}

	return nil
}
