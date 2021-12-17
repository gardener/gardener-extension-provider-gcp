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

package utils_test

import (
	. "github.com/gardener/gardener-extension-provider-gcp/pkg/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"

	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Utils", func() {

	DescribeTable("#HasSecretKey",
		func(secret *corev1.Secret, key string, matcher gomegatypes.GomegaMatcher) {
			Expect(HasSecretKey(secret, key)).To(matcher)
		},
		Entry("should return true as secret does contain key",
			&corev1.Secret{Data: map[string][]byte{"key-a": []byte("test")}}, "key-a", BeTrue()),
		Entry("should return false as secret does not contain key",
			&corev1.Secret{Data: map[string][]byte{"key-a": []byte("test")}}, "key-b", BeFalse()),
	)
})
