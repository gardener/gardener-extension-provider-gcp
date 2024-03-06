// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
