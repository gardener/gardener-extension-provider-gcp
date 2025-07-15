// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"
	"encoding/json"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/admission/validator"
	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	apisgcpv1alpha1 "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
)

var _ = Describe("Shoot validator", func() {
	Describe("#Validate", func() {
		const namespace = "garden-dev"

		var (
			shootValidator extensionswebhook.Validator

			ctrl         *gomock.Controller
			c            *mockclient.MockClient
			mgr          *mockmanager.MockManager
			cloudProfile *gardencorev1beta1.CloudProfile
			shoot        *core.Shoot

			ctx = context.Background()
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())

			scheme := runtime.NewScheme()
			Expect(apisgcp.AddToScheme(scheme)).To(Succeed())
			Expect(apisgcpv1alpha1.AddToScheme(scheme)).To(Succeed())
			Expect(gardencorev1beta1.AddToScheme(scheme)).To(Succeed())

			c = mockclient.NewMockClient(ctrl)

			mgr = mockmanager.NewMockManager(ctrl)
			mgr.EXPECT().GetScheme().Return(scheme).Times(2)
			mgr.EXPECT().GetClient().Return(c)
			shootValidator = validator.NewShootValidator(mgr)

			cloudProfile = &gardencorev1beta1.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "gcp",
				},
				Spec: gardencorev1beta1.CloudProfileSpec{
					Regions: []gardencorev1beta1.Region{
						{
							Name:  "us-west",
							Zones: []gardencorev1beta1.AvailabilityZone{{Name: "zone1"}},
						},
					},
					ProviderConfig: &runtime.RawExtension{
						Raw: encode(&apisgcpv1alpha1.CloudProfileConfig{
							TypeMeta: metav1.TypeMeta{
								APIVersion: apisgcpv1alpha1.SchemeGroupVersion.String(),
								Kind:       "CloudProfileConfig",
							},
						}),
					},
				},
			}

			shoot = &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: namespace,
				},
				Spec: core.ShootSpec{
					CloudProfile: &core.CloudProfileReference{
						Name: "cloudProfile",
					},
					Provider: core.Provider{
						Type:    "gcp",
						Workers: []core.Worker{},
					},
					Region: "us-west",
					Networking: &core.Networking{
						Nodes: ptr.To("10.250.0.0/16"),
					},
					Kubernetes: core.Kubernetes{
						Version: "1.31.17",
					},
				},
			}
		})

		Context("Workerless Shoot", func() {
			BeforeEach(func() {
				shoot.Spec.Provider.Workers = nil
			})

			It("should not validate", func() {
				err := shootValidator.Validate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("Shoot with workers", func() {
			BeforeEach(func() {
				shoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "CloudProfile",
					Name: "gcp",
				}
				shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{
					Raw: encode(&apisgcpv1alpha1.InfrastructureConfig{
						TypeMeta: metav1.TypeMeta{
							APIVersion: apisgcpv1alpha1.SchemeGroupVersion.String(),
							Kind:       "InfrastructureConfig",
						},
						Networks: apisgcpv1alpha1.NetworkConfig{
							Workers: "10.250.0.0/16",
						},
					}),
				}
				shoot.Spec.Provider.ControlPlaneConfig = &runtime.RawExtension{
					Raw: encode(&apisgcpv1alpha1.ControlPlaneConfig{
						TypeMeta: metav1.TypeMeta{
							APIVersion: apisgcpv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ControlPlaneConfig",
						},
						Zone: "zone1",
					}),
				}
				shoot.Spec.Provider.Workers = []core.Worker{{
					Name: "worker-1",
					Volume: &core.Volume{
						VolumeSize: "50Gi",
						Type:       ptr.To("pd-standard"),
					},
					Zones: []string{"zone1"},
				}}
			})

			It("should return err when networking is invalid", func() {
				c.EXPECT().Get(ctx, client.ObjectKey{Name: "gcp"}, &gardencorev1beta1.CloudProfile{}).SetArg(2, *cloudProfile)

				shoot.Spec.Networking.Nodes = nil
				shoot.Spec.Networking.IPFamilies = []core.IPFamily{core.IPFamilyIPv4, core.IPFamilyIPv6}
				err := shootValidator.Validate(ctx, shoot, nil)
				Expect(err).To(
					ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeRequired),
							"Field": Equal("spec.networking.nodes"),
						})),
					),
				)
			})

			It("should return err with IPv6-only networking", func() {
				c.EXPECT().Get(ctx, client.ObjectKey{Name: "gcp"}, &gardencorev1beta1.CloudProfile{}).SetArg(2, *cloudProfile)

				shoot.Spec.Networking.IPFamilies = []core.IPFamily{core.IPFamilyIPv6}

				err := shootValidator.Validate(ctx, shoot, nil)
				Expect(err).To(
					ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.networking.ipFamilies"),
						})),
					),
				)
			})
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}
