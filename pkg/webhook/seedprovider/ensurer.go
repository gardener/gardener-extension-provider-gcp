// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seedprovider

import (
	"context"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/config"
)

// NewEnsurer creates a new seedprovider ensurer.
func NewEnsurer(etcdStorage *config.ETCDStorage, logger logr.Logger) genericmutator.Ensurer {
	return &ensurer{
		etcdStorage: etcdStorage,
		logger:      logger.WithName("gcp-seedprovider-ensurer"),
	}
}

type ensurer struct {
	genericmutator.NoopEnsurer
	etcdStorage *config.ETCDStorage
	logger      logr.Logger
}

// EnsureETCD ensures that the etcd conform to the provider requirements.
func (e *ensurer) EnsureETCD(_ context.Context, _ gcontext.GardenContext, newEtcd, _ *druidv1alpha1.Etcd) error {
	capacity := resource.MustParse("10Gi")
	class := ""

	if newEtcd.Name == v1beta1constants.ETCDMain && e.etcdStorage != nil {
		if e.etcdStorage.Capacity != nil {
			capacity = *e.etcdStorage.Capacity
		}
		if e.etcdStorage.ClassName != nil {
			class = *e.etcdStorage.ClassName
		}
	}

	newEtcd.Spec.StorageClass = &class
	newEtcd.Spec.StorageCapacity = &capacity

	return nil
}
