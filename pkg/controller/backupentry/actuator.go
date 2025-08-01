// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupentry

import (
	"context"
	"fmt"
	"strings"

	"github.com/gardener/gardener/extensions/pkg/controller/backupentry/genericactuator"
	"github.com/gardener/gardener/extensions/pkg/util"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client"
)

type actuator struct {
	client client.Client
}

func newActuator(mgr manager.Manager) genericactuator.BackupEntryDelegate {
	return &actuator{
		client: mgr.GetClient(),
	}
}

func (a *actuator) GetETCDSecretData(_ context.Context, _ logr.Logger, _ *extensionsv1alpha1.BackupEntry, backupSecretData map[string][]byte) (map[string][]byte, error) {
	return backupSecretData, nil
}

func (a *actuator) Delete(ctx context.Context, _ logr.Logger, be *extensionsv1alpha1.BackupEntry) error {
	storageClient, err := gcpclient.NewStorageClientFromSecretRef(ctx, a.client, be.Spec.SecretRef)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}
	entryName := strings.TrimPrefix(be.Name, v1beta1constants.BackupSourcePrefix+"-")
	return util.DetermineError(storageClient.DeleteObjectsWithPrefix(ctx, be.Spec.BucketName, fmt.Sprintf("%s/", entryName)), helper.KnownCodes)
}
