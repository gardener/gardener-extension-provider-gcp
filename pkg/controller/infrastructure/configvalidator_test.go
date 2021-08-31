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

package infrastructure_test

import (
	"context"
	"encoding/json"
	"errors"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	. "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	mockgcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client/mock"

	"github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

const (
	name      = "infrastructure"
	namespace = "shoot--foobar--gcp"
	region    = "europe-west1"
)

var _ = Describe("ConfigValidator", func() {
	var (
		ctrl             *gomock.Controller
		c                *mockclient.MockClient
		gcpClientFactory *mockgcpclient.MockFactory
		gcpComputeClient *mockgcpclient.MockComputeClient
		ctx              context.Context
		logger           logr.Logger
		cv               infrastructure.ConfigValidator
		infra            *extensionsv1alpha1.Infrastructure
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)
		gcpClientFactory = mockgcpclient.NewMockFactory(ctrl)
		gcpComputeClient = mockgcpclient.NewMockComputeClient(ctrl)

		ctx = context.TODO()
		logger = log.Log.WithName("test")

		cv = NewConfigValidator(gcpClientFactory, logger)
		err := cv.(inject.Client).InjectClient(c)
		Expect(err).NotTo(HaveOccurred())

		infra = &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: extensionsv1alpha1.InfrastructureSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: gcp.Type,
					ProviderConfig: &runtime.RawExtension{
						Raw: encode(&apisgcp.InfrastructureConfig{
							Networks: apisgcp.NetworkConfig{
								CloudNAT: &apisgcp.CloudNAT{
									NatIPNames: []apisgcp.NatIPName{
										{Name: "test1"},
										{Name: "test2"},
									},
								},
							},
						}),
					},
				},
				Region: region,
				SecretRef: corev1.SecretReference{
					Name:      name,
					Namespace: namespace,
				},
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Validate", func() {
		BeforeEach(func() {
			gcpClientFactory.EXPECT().NewComputeClient(ctx, c, infra.Spec.SecretRef).Return(gcpComputeClient, nil)
		})

		It("should forbid NAT IP names that don't exist or are not available", func() {
			gcpComputeClient.EXPECT().GetExternalAddresses(ctx, region).Return(map[string]bool{
				"test2": false,
			}, nil)

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeNotFound),
				"Field": Equal("networks.cloudNAT.natIPNames[0].name"),
			}, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("networks.cloudNAT.natIPNames[1].name"),
				"Detail": Equal("external IP address is already in use"),
			}))
		})

		It("should allow NAT IP names that exist and are available", func() {
			gcpComputeClient.EXPECT().GetExternalAddresses(ctx, region).Return(map[string]bool{
				"test1": true,
				"test2": true,
			}, nil)

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(BeEmpty())
		})

		It("should fail with InternalError if getting external addresses failed", func() {
			gcpComputeClient.EXPECT().GetExternalAddresses(ctx, region).Return(nil, errors.New("test"))

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":   Equal(field.ErrorTypeInternal),
				"Field":  Equal("networks.cloudNAT"),
				"Detail": Equal("could not get external IP addresses: test"),
			}))
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}
