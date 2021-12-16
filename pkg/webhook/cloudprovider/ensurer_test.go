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

package cloudprovider_test

import (
	"context"
	"testing"

	. "github.com/gardener/gardener-extension-provider-gcp/pkg/webhook/cloudprovider"

	"github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CloudProvider Webhook Suite")
}

var _ = Describe("Ensurer", func() {
	var (
		logger  = log.Log.WithName("gcp-cloudprovider-webhook-test")
		ctx     = context.TODO()
		ensurer cloudprovider.Ensurer

		ctrl *gomock.Controller
		c    *mockclient.MockClient

		secret               *corev1.Secret
		serviceAccountData   string
		serviceAccountSecret corev1.Secret

		gctx          = gcontext.NewGardenContext(nil, nil)
		labelSelector = client.MatchingLabels{"gcp.provider.extensions.gardener.cloud/purpose": "service-account-secret"}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		ensurer = NewEnsurer(logger)

		err := ensurer.(inject.Client).InjectClient(c)
		Expect(err).NotTo(HaveOccurred())

		secret = &corev1.Secret{
			Data: map[string][]byte{
				"projectID": []byte("gcp-project-id"),
				"orgID":     []byte("gcp-project-id"),
			},
		}

		serviceAccountData = `{"key":"test"}`
		serviceAccountSecret = corev1.Secret{
			Data: map[string][]byte{
				"projectID":           []byte("gcp-project-id"),
				"orgID":               []byte("gcp-project-id"),
				"serviceaccount.json": []byte(serviceAccountData),
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#EnsureCloudProviderSecret", func() {
		It("should pass as a service account is present", func() {
			secret.Data["serviceaccount.json"] = []byte("some-service-account-data")

			err := ensurer.EnsureCloudProviderSecret(ctx, gctx, secret, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail as secret does not contain a project id", func() {
			delete(secret.Data, "projectID")

			err := ensurer.EnsureCloudProviderSecret(ctx, gctx, secret, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should fail as secret does not contain a organisation id", func() {
			delete(secret.Data, "orgID")

			err := ensurer.EnsureCloudProviderSecret(ctx, gctx, secret, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should add service account", func() {
			c.EXPECT().List(gomock.Any(), &corev1.SecretList{}, labelSelector).
				DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = []corev1.Secret{serviceAccountSecret}
					return nil
				})

			err := ensurer.EnsureCloudProviderSecret(ctx, gctx, secret, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(secret.Data).To(Equal(map[string][]byte{
				"projectID":           []byte("gcp-project-id"),
				"orgID":               []byte("gcp-project-id"),
				"serviceaccount.json": []byte(`{"key":"test","project_id":"gcp-project-id"}`),
			}))
		})

		It("should add service account, but not consider one of the service account secrets as contain no organisation id", func() {
			serviceAccountSecret2 := serviceAccountSecret.DeepCopy()
			delete(serviceAccountSecret2.Data, "orgID")

			c.EXPECT().List(gomock.Any(), &corev1.SecretList{}, labelSelector).
				DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = []corev1.Secret{serviceAccountSecret, *serviceAccountSecret2}
					return nil
				})

			err := ensurer.EnsureCloudProviderSecret(ctx, gctx, secret, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(secret.Data).To(Equal(map[string][]byte{
				"projectID":           []byte("gcp-project-id"),
				"orgID":               []byte("gcp-project-id"),
				"serviceaccount.json": []byte(`{"key":"test","project_id":"gcp-project-id"}`),
			}))
		})

		It("should fail as no matching service account secrets exists", func() {
			c.EXPECT().List(gomock.Any(), &corev1.SecretList{}, labelSelector).
				DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = []corev1.Secret{}
					return nil
				})

			err := ensurer.EnsureCloudProviderSecret(ctx, gctx, secret, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should fail as more than one matching service account secret exists", func() {
			serviceAccountSecret2 := serviceAccountSecret.DeepCopy()

			c.EXPECT().List(gomock.Any(), &corev1.SecretList{}, labelSelector).
				DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = []corev1.Secret{serviceAccountSecret, *serviceAccountSecret2}
					return nil
				})

			err := ensurer.EnsureCloudProviderSecret(ctx, gctx, secret, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should fail as the matching service account secret contains no service account information", func() {
			delete(serviceAccountSecret.Data, "serviceaccount.json")

			c.EXPECT().List(gomock.Any(), &corev1.SecretList{}, labelSelector).
				DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = []corev1.Secret{serviceAccountSecret}
					return nil
				})

			err := ensurer.EnsureCloudProviderSecret(ctx, gctx, secret, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should fail as the service account cannot be calculated due to invalid input", func() {
			serviceAccountSecret.Data["serviceaccount.json"] = []byte("invalid-service-account-json-input")

			c.EXPECT().List(gomock.Any(), &corev1.SecretList{}, labelSelector).
				DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = []corev1.Secret{serviceAccountSecret}
					return nil
				})

			err := ensurer.EnsureCloudProviderSecret(ctx, gctx, secret, nil)
			Expect(err).To(HaveOccurred())
		})
	})
})
