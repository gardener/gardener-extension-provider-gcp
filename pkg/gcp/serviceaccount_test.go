// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"context"
	"fmt"

	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
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
			sa, err := GetServiceAccountFromJSON(serviceAccountData)
			Expect(err).NotTo(HaveOccurred())
			Expect(sa.ProjectID).To(Equal(projectID))
		})

		It("should error if the project ID is empty", func() {
			_, err := GetServiceAccountFromJSON([]byte(`{"project_id": ""`))

			Expect(err).To(HaveOccurred())
		})

		It("should error on malformed json", func() {
			_, err := GetServiceAccountFromJSON([]byte(`{"project_id": ""`))

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
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, actual *corev1.Secret, _ ...client.GetOption) error {
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
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, actual *corev1.Secret, _ ...client.GetOption) error {
					*actual = *secret
					return nil
				})

			actual, err := GetServiceAccountFromSecretReference(ctx, c, secretRef)

			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(Equal(serviceAccount))
		})
	})
})
