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

	. "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/validation"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
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
		Entry("should succeed when valid serviceaccount.json is present",
			map[string][]byte{"serviceaccount.json": []byte(`{"project_id": "my-project"}`)},
			BeNil()),
		Entry("should return error when serviceaccount.json does not contain a project ID",
			map[string][]byte{"serviceaccount.json": []byte(`{"foo": "bar"}`)}, HaveOccurred()),
		Entry("should return error when the project ID of the serviceaccount.json starts with a digit",
			map[string][]byte{"serviceaccount.json": []byte(`{"project_id": "0my-project"}`)},
			HaveOccurred()),
		Entry("should return error when the project ID of the serviceaccount.json ends with hyphen",
			map[string][]byte{"serviceaccount.json": []byte(`{"project_id": "my-project-"}`)},
			HaveOccurred()),
		Entry("should return error when the project ID of the serviceaccount.json is too short",
			map[string][]byte{"serviceaccount.json": []byte(`{"project_id": "foo"}`)},
			HaveOccurred()),
		Entry("should return error when the project ID of the serviceaccount.json is too long",
			map[string][]byte{"serviceaccount.json": []byte(fmt.Sprintf(`{"project_id": "%s"}`, strings.Repeat("a", 31)))},
			HaveOccurred()),

		Entry("should succeed when projectID and orgID are present",
			map[string][]byte{"projectID": []byte("gcp-project-id"), "orgID": []byte("gcp-org-id")}, BeNil()),
		Entry("should return error when no serviceaccount.json or projectID plus orgID are present",
			map[string][]byte{}, HaveOccurred()),
		Entry("should return error when no serviceaccount.json and only a projectID is present",
			map[string][]byte{"projectID": []byte("gcp-project-id")}, HaveOccurred()),
		Entry("should return error as no serviceaccount.json and only a orgID is present",
			map[string][]byte{"orgID": []byte("gcp-org-id")}, HaveOccurred()),
	)
})
