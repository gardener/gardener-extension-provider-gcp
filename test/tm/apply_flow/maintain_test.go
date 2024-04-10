// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apply_flow_test

import (
	"context"
	"flag"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/test/framework"

	gcpinternal "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

var shootName = flag.String("shoot-name", "", "name of the shoot")

func init() {
	framework.RegisterGardenerFrameworkFlags()
}

var _ = Describe("Shoot maintenance testing", func() {
	f := framework.NewGardenerFramework(nil)

	framework.CIt("Testing if Shoot maintainance is successful with flow annotations", func(ctx context.Context) {
		shoot := &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Namespace: f.ProjectNamespace, Name: *shootName}}
		Expect(f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: f.ProjectNamespace, Name: *shootName}, shoot)).To(Succeed())

		Expect(f.UpdateShoot(ctx, shoot, func(shoot *gardencorev1beta1.Shoot) error {
			shoot.Annotations[gcpinternal.AnnotationKeyUseFlow] = "true"
			shoot.Annotations[v1beta1constants.GardenerOperation] = v1beta1constants.ShootOperationMaintain
			return nil
		})).To(Succeed())
	}, 1*time.Hour)
})
