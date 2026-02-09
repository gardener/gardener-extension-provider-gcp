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
	securityv1alpha1constants "github.com/gardener/gardener/pkg/apis/security/v1alpha1/constants"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client"
)

type actuator struct {
	reader client.Reader
}

var _ genericactuator.BackupEntryDelegate = (*actuator)(nil)

func newActuator(mgr manager.Manager) genericactuator.BackupEntryDelegate {
	return &actuator{
		reader: mgr.GetClient(),
	}
}

func (a *actuator) GetETCDSecretData(ctx context.Context, _ logr.Logger, be *extensionsv1alpha1.BackupEntry, backupSecretData map[string][]byte) (map[string][]byte, error) {
	if err := a.injectWorkloadIdentityData(ctx, be, backupSecretData); err != nil {
		return nil, err
	}
	return backupSecretData, nil
}

func (a *actuator) Delete(ctx context.Context, _ logr.Logger, be *extensionsv1alpha1.BackupEntry) error {
	storageClient, err := gcpclient.NewStorageClientFromSecretRef(ctx, a.reader, be.Spec.SecretRef)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}
	entryName := strings.TrimPrefix(be.Name, v1beta1constants.BackupSourcePrefix+"-")
	return util.DetermineError(storageClient.DeleteObjectsWithPrefix(ctx, be.Spec.BucketName, fmt.Sprintf("%s/", entryName)), helper.KnownCodes)
}

func (a *actuator) injectWorkloadIdentityData(ctx context.Context, be *extensionsv1alpha1.BackupEntry, data map[string][]byte) error {
	backupEntrySecret := &corev1.Secret{}
	if err := a.reader.Get(ctx, kutil.ObjectKeyFromSecretRef(be.Spec.SecretRef), backupEntrySecret); err != nil {
		return err
	}

	if !gcp.IsWorkloadIdentitySecret(backupEntrySecret) {
		return nil
	}

	if _, ok := backupEntrySecret.Data[securityv1alpha1constants.DataKeyConfig]; !ok {
		return fmt.Errorf("backupEntrySecret secret %q is missing a 'config' data key", kutil.ObjectKeyFromSecretRef(be.Spec.SecretRef))
	}

	backupEntrySecretConfig := map[string][]byte{securityv1alpha1constants.DataKeyConfig: backupEntrySecret.Data[securityv1alpha1constants.DataKeyConfig]}
	if err := gcp.SetWorkloadIdentityFeatures(backupEntrySecretConfig, getTokenMountDir(be)); err != nil {
		return err
	}

	data[gcp.ProjectIDField] = backupEntrySecretConfig[gcp.ProjectIDField]
	data[gcp.CredentialsConfigField] = backupEntrySecretConfig[gcp.CredentialsConfigField]

	// Etcd druid always sets the configuration environment variable `(SOURCE_)GOOGLE_APPLICATION_CREDENTIALS` to point to
	// `serviceaccount.json` file and then etcd-backup-restore must use it, therefore it cannot make use of the `credentialsConfig` file.
	// Instead, let's ensure that `serviceaccount.json` file exist and it has the content of the `credentialsConfig` here.
	// ref: https://github.com/gardener/etcd-druid/blob/afa2483c78867b29a7c0b44d73aa6c976d0c0773/internal/controller/etcdcopybackupstask/reconciler.go#L525-L526
	data[gcp.ServiceAccountJSONField] = data[gcp.CredentialsConfigField]

	return nil
}

func getTokenMountDir(be *extensionsv1alpha1.BackupEntry) string {
	const (
		sourceTokenMountDir = "/var/.source-gcp" // #nosec: G101 - This is just dirpath where the credentials file will be available.
		targetTokenMountDir = "/var/.gcp"        // #nosec: G101 - This is just dirpath where the credentials file will be available.
	)

	if isSourceBackupEntry(be) {
		return sourceTokenMountDir
	}
	return targetTokenMountDir
}

func isSourceBackupEntry(be *extensionsv1alpha1.BackupEntry) bool {
	return strings.HasPrefix(be.Name, v1beta1constants.BackupSourcePrefix+"-")
}
