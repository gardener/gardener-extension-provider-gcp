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

package validation_test

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"

	. "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/validation"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

var _ = Describe("Secret validation", func() {

	DescribeTable("#ValidateCloudProviderSecret",
		func(data map[string][]byte, matcher gomegatypes.GomegaMatcher) {
			secret := &corev1.Secret{
				Data: data,
			}
			err := ValidateCloudProviderSecret(secret)

			Expect(err).To(matcher)
		},
		Entry("should return error when the serviceaccount.json field is missing",
			map[string][]byte{}, HaveOccurred()),
		Entry("should return error when the project ID is missing",
			map[string][]byte{gcp.ServiceAccountJSONField: []byte(`{"foo": "bar"}`)}, HaveOccurred()),
		Entry("should return error when the project ID starts with a digit",
			map[string][]byte{gcp.ServiceAccountJSONField: []byte(`{"project_id": "0my-project", "type": "service_account"}`)},
			HaveOccurred()),
		Entry("should return error when the project ID ends with hyphen",
			map[string][]byte{gcp.ServiceAccountJSONField: []byte(`{"project_id": "my-project-", "type": "service_account"}`)},
			HaveOccurred()),
		Entry("should return error when the project ID is too short",
			map[string][]byte{gcp.ServiceAccountJSONField: []byte(`{"project_id": "foo", "type": "service_account"}`)},
			HaveOccurred()),
		Entry("should return error when the project ID is too long",
			map[string][]byte{gcp.ServiceAccountJSONField: []byte(fmt.Sprintf(`{"project_id": "%s", "type": "service_account"}`, strings.Repeat("a", 31)))},
			HaveOccurred()),
		Entry("should succeed when the credential type and project ID is valid",
			map[string][]byte{gcp.ServiceAccountJSONField: []byte(`{"project_id": "my-project", "type": "service_account"}`)},
			BeNil()),
		Entry("should fail when the credential type is in not in the allowed list",
			map[string][]byte{gcp.ServiceAccountJSONField: []byte(`{"project_id": "my-project", "type": "service_account"}`)},
			BeNil()),
	)
})
