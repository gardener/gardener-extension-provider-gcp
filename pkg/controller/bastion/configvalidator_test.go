package bastion

import (
	"context"
	"encoding/json"

	"github.com/gardener/gardener/extensions/pkg/controller/bastion"
	corev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	compute "google.golang.org/api/compute/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	mockgcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client/mock"
)

const (
	name      = "bastion"
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
		cv               bastion.ConfigValidator
		bastion          *extensionsv1alpha1.Bastion
		cluster          *extensions.Cluster
		secretBinding    *corev1beta1.SecretBinding
		worker           *extensionsv1alpha1.Worker
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)
		gcpClientFactory = mockgcpclient.NewMockFactory(ctrl)
		gcpComputeClient = mockgcpclient.NewMockComputeClient(ctrl)

		ctx = context.TODO()
		logger = log.Log.WithName("test")

		cv = NewConfigValidator(logger, gcpClientFactory)
		err := cv.(inject.Client).InjectClient(c)
		Expect(err).NotTo(HaveOccurred())

		bastion = &extensionsv1alpha1.Bastion{}
		cluster = &extensions.Cluster{}

		secretBinding = &corev1beta1.SecretBinding{
			SecretRef: corev1.SecretReference{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: namespace,
			},
		}

		infraStatus := &apisgcp.InfrastructureStatus{
			Networks: apisgcp.NetworkStatus{
				VPC: apisgcp.VPC{
					Name: name,
					CloudRouter: &apisgcp.CloudRouter{
						Name: name,
					},
				},
				Subnets: []apisgcp.Subnet{{
					Name:    name,
					Purpose: apisgcp.PurposeNodes,
				}},
				NatIPs: []apisgcp.NatIP{},
			},
		}

		worker = &extensionsv1alpha1.Worker{
			Spec: extensionsv1alpha1.WorkerSpec{
				InfrastructureProviderStatus: &runtime.RawExtension{
					Raw: encode(infraStatus),
				},
			},
		}

	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Validate", func() {
		BeforeEach(func() {
			cluster = createClusters()
			key := client.ObjectKey{Namespace: cluster.ObjectMeta.Name, Name: cluster.Shoot.Name}
			c.EXPECT().Get(ctx, key, &extensionsv1alpha1.Worker{}).DoAndReturn(
				func(_ context.Context, namespacedName client.ObjectKey, obj *extensionsv1alpha1.Worker, _ ...client.GetOption) error {
					worker.DeepCopyInto(obj)
					return nil
				})
			gcpClientFactory.EXPECT().Compute(ctx, c, secretBinding.SecretRef).Return(gcpComputeClient, nil)
		})

		It("should succeed if there are infrastructureStatus passed", func() {
			gcpComputeClient.EXPECT().GetNetwork(ctx, name).Return(&compute.Network{Name: name}, nil)
			gcpComputeClient.EXPECT().GetSubnet(ctx, region, name).Return(&compute.Subnetwork{Name: name}, nil)
			errorList := cv.Validate(ctx, bastion, cluster)
			Expect(errorList).To(BeEmpty())
		})

		It("should fail with InternalError if getting vpc failed", func() {
			gcpComputeClient.EXPECT().GetNetwork(ctx, name).Return(nil, nil)
			errorList := cv.Validate(ctx, bastion, cluster)
			Expect(errorList).To(ConsistOfFields(
				gstruct.Fields{
					"Type":   Equal(field.ErrorTypeInternal),
					"Field":  Equal("vpc"),
					"Detail": Equal("could not get vpc bastion from gcp provider: Not Found"),
				}))
		})

		It("should fail with InternalError if getting subnet failed", func() {
			gcpComputeClient.EXPECT().GetNetwork(ctx, name).Return(&compute.Network{Name: name}, nil)
			gcpComputeClient.EXPECT().GetSubnet(ctx, region, name).Return(nil, nil)
			errorList := cv.Validate(ctx, bastion, cluster)
			Expect(errorList).To(ConsistOfFields(
				gstruct.Fields{
					"Type":   Equal(field.ErrorTypeInternal),
					"Field":  Equal("subnet"),
					"Detail": Equal("could not get subnet bastion from gcp provider: Not Found"),
				}))
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}

func createClusters() *extensions.Cluster {
	return &extensions.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
		Shoot: &corev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name: v1beta1constants.SecretNameCloudProvider,
			},
			Spec: corev1beta1.ShootSpec{
				Region: region,
			},
		},
	}
}
