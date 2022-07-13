// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gcp

import (
	"context"
	"fmt"

	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Service Account", func() {
	var (
		projectID          string
		email              string
		serviceAccountData []byte
		serviceAccount     *ServiceAccount
		secret             *corev1.Secret
	)
	BeforeEach(func() {
		projectID = "project"
		email = "email"
		serviceAccountData = []byte(fmt.Sprintf(`{"project_id": "%s", "client_email": "%s"}`, projectID, email))
		serviceAccount = &ServiceAccount{ProjectID: projectID, Email: email, Raw: serviceAccountData}
		secret = &corev1.Secret{
			Data: map[string][]byte{
				ServiceAccountJSONField: serviceAccountData,
			},
		}
	})

	var ctrl *gomock.Controller
	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})
	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#ExtractServiceAccountProjectID", func() {
		It("should correctly extract the project ID", func() {
			actualProjectID, err := ExtractServiceAccountProjectID(serviceAccountData)

			Expect(err).NotTo(HaveOccurred())
			Expect(actualProjectID).To(Equal(projectID))
		})

		It("should error if the project ID is empty", func() {
			_, err := ExtractServiceAccountProjectID([]byte(`{"project_id": ""`))

			Expect(err).To(HaveOccurred())
		})

		It("should error on malformed json", func() {
			_, err := ExtractServiceAccountProjectID([]byte(`{"project_id: "foo"}"`))

			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#ReadServiceAccountSecret", func() {
		It("should read the service account data from the secret", func() {
			secret := &corev1.Secret{Data: map[string][]byte{
				ServiceAccountJSONField: serviceAccountData,
			}}

			actual, err := GetServiceAccountFromSecret(secret)
			Expect(err).NotTo(HaveOccurred())
			Expect(actual.Raw).To(Equal(serviceAccountData))
		})
	})

	Describe("#GetServiceAccountData", func() {
		It("should retrieve the service account data", func() {
			var (
				c         = mockclient.NewMockClient(ctrl)
				ctx       = context.TODO()
				namespace = "foo"
				name      = "bar"
				secretRef = corev1.SecretReference{
					Namespace: namespace,
					Name:      name,
				}
			)
			c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, actual *corev1.Secret) error {
					*actual = *secret
					return nil
				})

			actual, err := GetServiceAccountFromSecretReference(ctx, c, secretRef)

			Expect(err).NotTo(HaveOccurred())
			Expect(actual.Raw).To(Equal(serviceAccountData))
		})
	})

	Describe("#GetServiceAccount", func() {
		It("should correctly retrieve the service account", func() {
			var (
				c         = mockclient.NewMockClient(ctrl)
				ctx       = context.TODO()
				namespace = "foo"
				name      = "bar"
				secretRef = corev1.SecretReference{
					Namespace: namespace,
					Name:      name,
				}
			)
			c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, actual *corev1.Secret) error {
					*actual = *secret
					return nil
				})

			actual, err := GetServiceAccountFromSecretReference(ctx, c, secretRef)

			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(Equal(serviceAccount))
		})
	})
})
