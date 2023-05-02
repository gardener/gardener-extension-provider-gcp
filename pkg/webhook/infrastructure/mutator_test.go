// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package infrastructure

import (
	"context"
	"testing"

	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

const (
	shootNamespace = "shoot--foo--bar"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Infrastructure Webhook Suite")
}

var _ = Describe("Mutate", func() {
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#UseFlowAnnotation", func() {
		var (
			mutator extensionswebhook.Mutator
			cluster *controller.Cluster
			ctx     context.Context
		)

		Context("create", func() {
			BeforeEach(func() {
				mutator = New(logger)
				ctx = context.TODO()
				c := mockclient.NewMockClient(ctrl)
				c.EXPECT().Get(ctx, client.ObjectKey{Name: shootNamespace}, gomock.AssignableToTypeOf(&extensionsv1alpha1.Cluster{})).
					DoAndReturn(
						func(_ context.Context, _ types.NamespacedName, obj *extensionsv1alpha1.Cluster, _ ...client.GetOption) error {
							seedJSON, err := json.Marshal(cluster.Seed)
							Expect(err).NotTo(HaveOccurred())
							*obj = extensionsv1alpha1.Cluster{
								ObjectMeta: cluster.ObjectMeta,
								Spec: extensionsv1alpha1.ClusterSpec{
									Seed: runtime.RawExtension{Raw: seedJSON},
								},
							}
							return nil
						})
				err := mutator.(inject.Client).InjectClient(c)
				Expect(err).NotTo(HaveOccurred())
				cluster = &controller.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: shootNamespace,
					},
					Seed: &gardencorev1beta1.Seed{
						ObjectMeta: metav1.ObjectMeta{
							Name:   shootNamespace,
							Labels: map[string]string{},
						},
					},
				}
			})

			It("should add use-flow annotation if seed label is set to new", func() {
				cluster.Seed.Labels[gcp.SeedLabelKeyUseFlow] = gcp.SeedLabelUseFlowValueNew
				newInfra := &extensionsv1alpha1.Infrastructure{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dummy",
						Namespace: shootNamespace,
					},
				}

				err := mutator.Mutate(ctx, newInfra, nil)

				Expect(err).To(BeNil())
				Expect(err).To(BeNil())
				Expect(newInfra.Annotations[gcp.AnnotationKeyUseFlow]).To(Equal("true"))
			})

			It("should do nothing if seed label is set to true", func() {
				cluster.Seed.Labels[gcp.SeedLabelKeyUseFlow] = "true"
				newInfra := &extensionsv1alpha1.Infrastructure{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dummy",
						Namespace: shootNamespace,
					},
				}
				err := mutator.Mutate(ctx, newInfra, nil)
				Expect(err).To(BeNil())
				Expect(newInfra.Annotations[gcp.AnnotationKeyUseFlow]).To(Equal(""))
			})
		})

		Context("update", func() {
			BeforeEach(func() {
				mutator = New(logger)
				cluster = &controller.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: shootNamespace,
					},
				}
			})

			It("should do nothing on update", func() {
				newInfra := &extensionsv1alpha1.Infrastructure{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dummy",
						Namespace: shootNamespace,
					},
				}
				err := mutator.Mutate(ctx, newInfra, newInfra)
				Expect(err).To(BeNil())
				Expect(newInfra.Annotations[gcp.AnnotationKeyUseFlow]).To(Equal(""))
			})
		})
	})
})
