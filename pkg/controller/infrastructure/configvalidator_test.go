// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure_test

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	infractrl "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	mockgcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client/mock"
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
		mgr              *mockmanager.MockManager
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)
		gcpClientFactory = mockgcpclient.NewMockFactory(ctrl)
		gcpComputeClient = mockgcpclient.NewMockComputeClient(ctrl)

		ctx = context.TODO()
		logger = log.Log.WithName("test")

		mgr = mockmanager.NewMockManager(ctrl)
		mgr.EXPECT().GetClient().Return(c)

		cv = infractrl.NewConfigValidator(mgr, logger, gcpClientFactory)

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
			gcpClientFactory.EXPECT().Compute(ctx, c, infra.Spec.SecretRef).Return(gcpComputeClient, nil)
		})

		It("should succeed if there are no NAT IP names", func() {
			infra.Spec.ProviderConfig.Raw = encode(&apisgcp.InfrastructureConfig{
				Networks: apisgcp.NetworkConfig{},
			})

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(BeEmpty())
		})

		It("should forbid NAT IP names that don't exist or are not available", func() {
			gcpComputeClient.EXPECT().GetExternalAddresses(ctx, region).Return(map[string][]string{
				"test2": {"foo"},
			}, nil)

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeNotFound),
				"Field": Equal("networks.cloudNAT.natIPNames[0].name"),
			}, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("networks.cloudNAT.natIPNames[1].name"),
				"Detail": Equal("external IP address is already in use by foo"),
			}))
		})

		It("should allow NAT IP names that exist and are available, or in use by the default cloud router", func() {
			gcpComputeClient.EXPECT().GetExternalAddresses(ctx, region).Return(map[string][]string{
				"test1": nil,
				"test2": {namespace + "-cloud-router"},
			}, nil)

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(BeEmpty())
		})

		It("should allow NAT IP names that exist and are in use by the configured cloud router", func() {
			infra.Spec.ProviderConfig.Raw = encode(&apisgcp.InfrastructureConfig{
				Networks: apisgcp.NetworkConfig{
					VPC: &apisgcp.VPC{
						Name: "test-vpc",
						CloudRouter: &apisgcp.CloudRouter{
							Name: "test-cloud-router",
						},
					},
					CloudNAT: &apisgcp.CloudNAT{
						NatIPNames: []apisgcp.NatIPName{
							{Name: "test1"},
						},
					},
				},
			})
			gcpComputeClient.EXPECT().GetExternalAddresses(ctx, region).Return(map[string][]string{
				"test1": {"test-cloud-router"},
			}, nil)

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(BeEmpty())
		})

		It("should fail with InternalError if getting external addresses failed", func() {
			gcpComputeClient.EXPECT().GetExternalAddresses(ctx, region).Return(nil, errors.New("test"))

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":   Equal(field.ErrorTypeInternal),
				"Field":  Equal("networks"),
				"Detail": Equal("could not get external IP addresses: test"),
			}))
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}
