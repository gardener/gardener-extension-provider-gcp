// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket_test

import (
	"context"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/api/option"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

func secretsFromEnv() {
	if len(*serviceAccount) == 0 {
		serviceAccount = ptr.To(os.Getenv("SERVICE_ACCOUNT"))
	}
	if len(*region) == 0 {
		region = ptr.To(os.Getenv("REGION"))
	}
}

func validateFlags() {
	if len(*serviceAccount) == 0 {
		panic("GCP service account required. Either provide it via the service-account flag or set the SERVICE_ACCOUNT environment variable")
	}
	if len(*region) == 0 {
		panic("GCP region required. Either provide it via the region flag or set the REGION environment variable")
	}
	if len(*logLevel) == 0 {
		logLevel = ptr.To("debug")
	} else {
		if *logLevel != "debug" && *logLevel != "info" && *logLevel != "error" {
			panic("Invalid log level: " + *logLevel)
		}
	}
}

func getStorageClient(ctx context.Context, serviceAccount string) *storage.Client {
	client, err := storage.NewClient(ctx, option.WithCredentialsJSON([]byte(serviceAccount)))
	Expect(err).NotTo(HaveOccurred(), "Failed to create GCP storage client")
	return client
}

func createNamespace(ctx context.Context, c client.Client, namespace *corev1.Namespace) {
	log.Info("Creating namespace", "namespace", namespace.Name)
	Expect(c.Create(ctx, namespace)).To(Succeed(), "Failed to create namespace: %s", namespace.Name)
}

func deleteNamespace(ctx context.Context, c client.Client, namespace *corev1.Namespace) {
	log.Info("Deleting namespace", "namespace", namespace.Name)
	Expect(client.IgnoreNotFound(c.Delete(ctx, namespace))).To(Succeed())
}

func ensureGardenNamespace(ctx context.Context, c client.Client) (*corev1.Namespace, bool) {
	gardenNamespaceAlreadyExists := false
	gardenNamespace := &corev1.Namespace{}
	err := c.Get(ctx, client.ObjectKey{Name: gardenNamespaceName}, gardenNamespace)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			log.Info("Garden namespace not found, creating it", "namespace", gardenNamespaceName)
			gardenNamespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: gardenNamespaceName,
				},
			}
			Expect(c.Create(ctx, gardenNamespace)).To(Succeed(), "Failed to create garden namespace")
		} else {
			log.Error(err, "Failed to check for garden namespace")
			Expect(err).NotTo(HaveOccurred(), "Unexpected error while checking for garden namespace")
		}
	} else {
		gardenNamespaceAlreadyExists = true
		log.Info("Garden namespace already exists", "namespace", gardenNamespaceName)
	}
	return gardenNamespace, gardenNamespaceAlreadyExists
}

func createBackupBucketSecret(ctx context.Context, c client.Client, secret *corev1.Secret) {
	log.Info("Creating secret", "name", secret.Name, "namespace", secret.Namespace)
	Expect(c.Create(ctx, secret)).To(Succeed(), "Failed to create secret: %s", secret.Name)
}

func deleteBackupBucketSecret(ctx context.Context, c client.Client, secret *corev1.Secret) {
	log.Info("Deleting secret", "name", secret.Name, "namespace", secret.Namespace)
	Expect(client.IgnoreNotFound(c.Delete(ctx, secret))).To(Succeed())
}

func createBackupBucket(ctx context.Context, c client.Client, backupBucket *v1alpha1.BackupBucket) {
	log.Info("Creating backupBucket", "backupBucket", backupBucket)
	Expect(c.Create(ctx, backupBucket)).To(Succeed(), "Failed to create backupBucket: %s", backupBucket.Name)
}

func deleteBackupBucket(ctx context.Context, c client.Client, backupBucket *v1alpha1.BackupBucket) {
	log.Info("Deleting backupBucket", "backupBucket", backupBucket)
	Expect(client.IgnoreNotFound(c.Delete(ctx, backupBucket))).To(Succeed())
}

func waitUntilBackupBucketReady(ctx context.Context, c client.Client, backupBucket *v1alpha1.BackupBucket) {
	err := extensions.WaitUntilExtensionObjectReady(
		ctx,
		c,
		log,
		backupBucket,
		v1alpha1.BackupBucketResource,
		10*time.Second,
		30*time.Second,
		5*time.Minute,
		nil,
	)
	if err != nil {
		log.Info("BackupBucket is not ready yet; this is expected during initial reconciliation", "error", err)
	}
	Expect(err).To(Succeed(), "BackupBucket did not become ready: %s", backupBucket.Name)
	log.Info("BackupBucket is ready", "backupBucket", backupBucket)
}

func waitUntilBackupBucketDeleted(ctx context.Context, c client.Client, backupBucket *v1alpha1.BackupBucket) {
	Expect(extensions.WaitUntilExtensionObjectDeleted(
		ctx,
		c,
		log,
		backupBucket.DeepCopy(),
		v1alpha1.BackupBucketResource,
		10*time.Second,
		5*time.Minute,
	)).To(Succeed())
	log.Info("BackupBucket successfully deleted", "backupBucket", backupBucket)
}

func getBackupBucketAndVerifyStatus(ctx context.Context, c client.Client, backupBucket *v1alpha1.BackupBucket) {
	log.Info("Verifying backupBucket", "backupBucket", backupBucket)
	Expect(c.Get(ctx, client.ObjectKey{Name: backupBucket.Name}, backupBucket)).To(Succeed())

	By("verifying LastOperation state")
	Expect(backupBucket.Status.LastOperation).NotTo(BeNil(), "LastOperation should not be nil")
	Expect(backupBucket.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded), "LastOperation state should be Succeeded")
	Expect(backupBucket.Status.LastOperation.Type).To(Equal(gardencorev1beta1.LastOperationTypeCreate), "LastOperation type should be Create")

	By("verifying GeneratedSecretRef")
	if backupBucket.Status.GeneratedSecretRef != nil {
		Expect(backupBucket.Status.GeneratedSecretRef.Name).NotTo(BeEmpty(), "GeneratedSecretRef name should not be empty")
		Expect(backupBucket.Status.GeneratedSecretRef.Namespace).NotTo(BeEmpty(), "GeneratedSecretRef namespace should not be empty")
	}
}

func verifyBackupBucket(ctx context.Context, storageClient *storage.Client, backupBucket *v1alpha1.BackupBucket) {
	bucketName := backupBucket.Name

	By("verifying GCS bucket")
	bucket := storageClient.Bucket(bucketName)
	attrs, err := bucket.Attrs(ctx)
	Expect(err).NotTo(HaveOccurred(), "Failed to verify GCS bucket existence")

	By("verifying GCS bucket location")
	Expect(attrs.Location).To(Equal(strings.ToUpper(backupBucket.Spec.Region)), "Bucket location does not match expected region")
}

func verifyBackupBucketDeleted(ctx context.Context, storageClient *storage.Client, backupBucket *v1alpha1.BackupBucket) {
	bucketName := backupBucket.Name

	By("verifying GCS bucket deletion")
	_, err := storageClient.Bucket(bucketName).Attrs(ctx)
	Expect(err).To(HaveOccurred(), "Expected GCS bucket to be deleted, but it still exists")
}

func newBackupBucket(name string, region string) *v1alpha1.BackupBucket {
	return &v1alpha1.BackupBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.BackupBucketSpec{
			DefaultSpec: v1alpha1.DefaultSpec{
				Type: gcp.Type,
			},
			Region: region,
			SecretRef: corev1.SecretReference{
				Name:      backupBucketSecretName,
				Namespace: name,
			},
		},
	}
}

func randomString() string {
	rs, err := gardenerutils.GenerateRandomStringFromCharset(5, "0123456789abcdefghijklmnopqrstuvwxyz")
	Expect(err).NotTo(HaveOccurred(), "Failed to generate random string")
	log.Info("Generated random string", "randomString", rs)
	return rs
}
