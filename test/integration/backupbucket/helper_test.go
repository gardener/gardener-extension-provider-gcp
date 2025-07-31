// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/api/option"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gcpv1alpha1 "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
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

func createBackupBucketSecret(ctx context.Context, c client.Client, secret *corev1.Secret) {
	log.Info("Creating secret", "name", secret.Name, "namespace", secret.Namespace)
	Expect(c.Create(ctx, secret)).To(Succeed(), "Failed to create secret: %s", secret.Name)
}

func deleteBackupBucketSecret(ctx context.Context, c client.Client, secret *corev1.Secret) {
	log.Info("Deleting secret", "name", secret.Name, "namespace", secret.Namespace)
	Expect(client.IgnoreNotFound(c.Delete(ctx, secret))).To(Succeed())
}

func createBackupBucket(ctx context.Context, c client.Client, backupBucket *extensionsv1alpha1.BackupBucket) {
	log.Info("Creating backupBucket", "backupBucket", backupBucket)
	Expect(c.Create(ctx, backupBucket)).To(Succeed(), "Failed to create backupBucket: %s", backupBucket.Name)
}

func updateBackupBucket(ctx context.Context, c client.Client, backupBucket *extensionsv1alpha1.BackupBucket) {
	log.Info("Updating backupBucket", "backupBucket", backupBucket)
	Expect(c.Update(ctx, backupBucket)).To(Succeed(), "Failed to update backupBucket: %s", backupBucket.Name)
}

func fetchBackupBucket(ctx context.Context, c client.Client, name string) *extensionsv1alpha1.BackupBucket {
	backupBucket := &extensionsv1alpha1.BackupBucket{}
	err := c.Get(ctx, client.ObjectKey{Name: name}, backupBucket)
	Expect(err).NotTo(HaveOccurred(), "Failed to fetch backupBucket from the cluster")
	return backupBucket
}

func deleteBackupBucket(ctx context.Context, c client.Client, backupBucket *extensionsv1alpha1.BackupBucket) {
	log.Info("Deleting backupBucket", "backupBucket", backupBucket)
	Expect(client.IgnoreNotFound(c.Delete(ctx, backupBucket))).To(Succeed())
}

func waitUntilBackupBucketReady(ctx context.Context, c client.Client, backupBucket *extensionsv1alpha1.BackupBucket) {
	Expect(extensions.WaitUntilExtensionObjectReady(
		ctx,
		c,
		log,
		backupBucket,
		extensionsv1alpha1.BackupBucketResource,
		10*time.Second,
		30*time.Second,
		5*time.Minute,
		nil,
	)).To(Succeed(), "BackupBucket did not become ready: %s", backupBucket.Name)
	log.Info("BackupBucket is ready", "backupBucket", backupBucket)
}

func waitUntilBackupBucketDeleted(ctx context.Context, c client.Client, backupBucket *extensionsv1alpha1.BackupBucket) {
	Expect(extensions.WaitUntilExtensionObjectDeleted(
		ctx,
		c,
		log,
		backupBucket.DeepCopy(),
		extensionsv1alpha1.BackupBucketResource,
		10*time.Second,
		5*time.Minute,
	)).To(Succeed())
	log.Info("BackupBucket successfully deleted", "backupBucket", backupBucket)
}

func newBackupBucket(name, region string, providerConfig *gcpv1alpha1.BackupBucketConfig) *extensionsv1alpha1.BackupBucket {
	var providerConfigRaw *runtime.RawExtension
	if providerConfig != nil {
		providerConfig.APIVersion = "gcp.provider.extensions.gardener.cloud/v1alpha1"
		providerConfig.Kind = "BackupBucketConfig"
		providerConfigJSON, err := json.Marshal(providerConfig)
		Expect(err).NotTo(HaveOccurred(), "Failed to marshal providerConfig to JSON")
		providerConfigRaw = &runtime.RawExtension{
			Raw: providerConfigJSON,
		}
		log.Info("Creating new backupBucket object", "region", region, "providerConfig", string(providerConfigJSON))
	} else {
		providerConfigRaw = &runtime.RawExtension{}
		log.Info("Creating new backupBucket object with empty providerConfig", "region", region)
	}

	return &extensionsv1alpha1.BackupBucket{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "extensions.gardener.cloud/v1alpha1",
			Kind:       "BackupBucket",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: extensionsv1alpha1.BackupBucketSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type:           gcp.Type,
				ProviderConfig: providerConfigRaw,
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

// functions for verification
func verifyBackupBucketAndStatus(ctx context.Context, c client.Client, storageClient *storage.Client, backupBucket *extensionsv1alpha1.BackupBucket) {
	By("getting backupbucket and verifying its status")
	verifyBackupBucketStatus(ctx, c, backupBucket)

	By("verifying that the GCS bucket exists and matches backupbucket")
	verifyBackupBucket(ctx, storageClient, backupBucket)
}

func verifyBackupBucketStatus(ctx context.Context, c client.Client, backupBucket *extensionsv1alpha1.BackupBucket) {
	log.Info("Verifying backupBucket", "backupBucket", backupBucket)
	By("fetching backupBucket from the cluster")
	backupBucket = fetchBackupBucket(ctx, c, backupBucket.Name)

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

func verifyBackupBucket(ctx context.Context, storageClient *storage.Client, backupBucket *extensionsv1alpha1.BackupBucket) {
	bucketName := backupBucket.Name

	By("verifying GCS bucket")
	bucket := storageClient.Bucket(bucketName)
	attrs, err := bucket.Attrs(ctx)
	Expect(err).NotTo(HaveOccurred(), "Failed to verify GCS bucket existence")

	By("verifying GCS bucket location")
	Expect(attrs.Location).To(Equal(strings.ToUpper(backupBucket.Spec.Region)), "Bucket location does not match expected region")
}

func verifyBackupBucketDeleted(ctx context.Context, storageClient *storage.Client, backupBucket *extensionsv1alpha1.BackupBucket) {
	bucketName := backupBucket.Name

	By("verifying GCS bucket deletion")
	_, err := storageClient.Bucket(bucketName).Attrs(ctx)
	Expect(err).To(HaveOccurred(), "Expected GCS bucket to be deleted, but it still exists")
}

func verifyImmutabilityPolicy(ctx context.Context, storageClient *storage.Client, backupBucket *extensionsv1alpha1.BackupBucket, immutabilityConfig *gcpv1alpha1.ImmutableConfig) {
	By("fetching immutability policy from Azure")
	attrs, err := storageClient.Bucket(backupBucket.Name).Attrs(ctx)

	Expect(err).NotTo(HaveOccurred(), "Failed to fetch immutability policy from GCS")

	By("verifying immutability policy configuration")
	Expect(int32(attrs.RetentionPolicy.RetentionPeriod.Hours())).To(Equal(int32(immutabilityConfig.RetentionPeriod.Hours())), "Retention period mismatch")
	Expect(attrs.RetentionPolicy.IsLocked).To(BeFalse(), "Immutability policy state mismatch")
}

func verifyBucketImmutability(ctx context.Context, c client.Client, storageClient *storage.Client, backupBucket *extensionsv1alpha1.BackupBucket) {
	bucketName := backupBucket.Name
	objectName := bucketName + "-test-object"

	defer func() {
		By("deleting immutability policy on backupBucket")
		backupBucket = fetchBackupBucket(ctx, c, backupBucket.Name)
		backupBucket.Spec.ProviderConfig = nil
		updateBackupBucket(ctx, c, backupBucket)

		By("deleting immutability policy on GCS bucket")
		_, err := storageClient.Bucket(bucketName).Update(ctx, storage.BucketAttrsToUpdate{
			RetentionPolicy: &storage.RetentionPolicy{},
		})
		Expect(err).NotTo(HaveOccurred(), "Failed to delete immutability policy on GCS bucket")

		By("deleting test object from GCS bucket")
		err = storageClient.Bucket(bucketName).Object(objectName).Delete(ctx)
		Expect(err).NotTo(HaveOccurred(), "Failed to delete test object from GCS bucket")
	}()

	By("creating a test object in the GCS bucket to verify immutability")
	storageWriter := storageClient.Bucket(bucketName).Object(objectName).NewWriter(ctx)
	storageWriter.ContentType = "text/plain"
	_, err := storageWriter.Write([]byte("This is a test object for immutability verification."))
	Expect(err).NotTo(HaveOccurred(), "Failed to write test object to GCS bucket")
	err = storageWriter.Close()
	Expect(err).NotTo(HaveOccurred(), "Failed to close storage writer after writing test object")

	By("attempting to overwrite the test object to verify immutability")
	storageWriter = storageClient.Bucket(bucketName).Object(objectName).NewWriter(ctx)
	storageWriter.ContentType = "text/plain"
	_, err = storageWriter.Write([]byte("This should fail if immutability is enforced."))
	// the actual write does not return an error, but closing the writer should fail due to the immutability policy
	Expect(err).NotTo(HaveOccurred())
	err = storageWriter.Close()
	Expect(err).To(HaveOccurred(), "Expected an error when trying to overwrite the test object due to immutability policy")
	log.Info("Expected error occurred when trying to overwrite the test object", "error", err)

	By("attempting to delete the test object to verify immutability")
	err = storageClient.Bucket(bucketName).Object(objectName).Delete(ctx)
	Expect(err).To(HaveOccurred(), "Expected an error when trying to delete the test object due to immutability policy")
	log.Info("Expected error occurred when trying to delete the test object", "error", err)
}

func verifyLockedImmutabilityPolicy(ctx context.Context, storageClient *storage.Client, backupBucket *extensionsv1alpha1.BackupBucket) {
	By("attempting to modify a locked immutability policy")
	bucket := storageClient.Bucket(backupBucket.Name)

	// Attempt to update the retention policy by decreasing the retention period (increasing is allowed)
	_, err := bucket.Update(ctx, storage.BucketAttrsToUpdate{
		RetentionPolicy: &storage.RetentionPolicy{
			RetentionPeriod: 20 * time.Hour,
			IsLocked:        true,
		},
	})
	Expect(err).To(HaveOccurred(), "Expected an error when trying to decrease a locked immutability policy's retention period")
	log.Info("Expected error occurred when trying to modify a locked immutability policy", "error", err)

	By("attempting to delete a locked immutability policy")
	_, err = bucket.Update(ctx, storage.BucketAttrsToUpdate{
		RetentionPolicy: &storage.RetentionPolicy{},
	})
	Expect(err).To(HaveOccurred(), "Expected an error when trying to delete a bucket's locked immutability policy")
	log.Info("Expected error occurred when trying to delete a locked immutability policy", "error", err)
}

func verifyRetentionPeriod(ctx context.Context, storageClient *storage.Client, backupBucket *extensionsv1alpha1.BackupBucket, retentionPeriod time.Duration) {
	bucketName := backupBucket.Name
	objectName := bucketName + "-test-object"

	By("creating a test object in the GCS bucket to verify the retention period")
	storageWriter := storageClient.Bucket(bucketName).Object(objectName).NewWriter(ctx)
	storageWriter.ContentType = "text/plain"
	_, err := storageWriter.Write([]byte("This is a test object for retention period verification."))
	Expect(err).NotTo(HaveOccurred(), "Failed to write test object to GCS bucket")
	err = storageWriter.Close()
	Expect(err).NotTo(HaveOccurred(), "Failed to close storage writer after writing test object")
	startTime := time.Now()

	By("attempting to delete the test object to verify immutability")
	err = storageClient.Bucket(bucketName).Object(objectName).Delete(ctx)
	Expect(err).To(HaveOccurred(), "Expected an error when trying to delete the test object due retention period not passed")
	log.Info("Expected error occurred when trying to delete the test object", "error", err)

	By("waiting for the retention period to pass before overwriting the test object")
	passedTime := time.Since(startTime)
	if passedTime < retentionPeriod {
		waitTime := retentionPeriod - passedTime
		log.Info("time to wait", "waitTime", waitTime)
		time.Sleep(waitTime)
	}

	By("verifying that the object can be overwritten after retention period has passed")
	storageWriter = storageClient.Bucket(bucketName).Object(objectName).NewWriter(ctx)
	storageWriter.ContentType = "text/plain"
	newContent := "This should be the new content after retention period."
	_, err = storageWriter.Write([]byte(newContent))
	Expect(err).NotTo(HaveOccurred(), "Failed to write new content to test object after retention period")
	err = storageWriter.Close()
	Expect(err).NotTo(HaveOccurred(), "Failed to close storage writer after writing new content to test object")
	log.Info("Expected error occurred when trying to overwrite the test object", "error", err)
	storageReader, err := storageClient.Bucket(bucketName).Object(objectName).NewReader(ctx)
	Expect(err).NotTo(HaveOccurred(), "Failed to create storage reader for test object with new content")
	readContent := make([]byte, len(newContent))
	_, err = storageReader.Read(readContent)
	storageReader.Close()
	Expect(err).NotTo(HaveOccurred(), "Failed to read content from test object with new content")
	Expect(string(readContent)).To(Equal(newContent), "Content of the test object after retention period does not match expected new content")
	startTime = time.Now()

	By("waiting for the retention period to pass before deleting the test object")
	passedTime = time.Since(startTime)
	if passedTime < retentionPeriod {
		waitTime := retentionPeriod - passedTime
		log.Info("time to wait", "waitTime", waitTime)
		time.Sleep(waitTime)
	}

	By("deleting the test object after retention period has passed")
	err = storageClient.Bucket(bucketName).Object(objectName).Delete(ctx)
	Expect(err).NotTo(HaveOccurred(), "Failed to delete test object from GCS bucket after retention period")
}
