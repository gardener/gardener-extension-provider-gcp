# Using the GCP provider extension with Gardener as operator

The [`core.gardener.cloud/v1beta1.CloudProfile` resource](https://github.com/gardener/gardener/blob/master/example/30-cloudprofile.yaml) declares a `providerConfig` field that is meant to contain provider-specific configuration.
The [`core.gardener.cloud/v1beta1.Seed` resource](https://github.com/gardener/gardener/blob/master/example/50-seed.yaml) is structured similarly.
Additionally, it allows configuring settings for the backups of the main etcds' data of shoot clusters control planes running in this seed cluster.

This document explains the necessary configuration for this provider extension.

## `CloudProfile` resource

This section describes, how the configuration for `CloudProfile`s looks like for GCP by providing an example `CloudProfile` manifest with minimal configuration that can be used to allow the creation of GCP shoot clusters.

### `CloudProfileConfig`

The cloud profile configuration contains information about the real machine image IDs in the GCP environment (image URLs).
You have to map every version that you specify in `.spec.machineImages[].versions` here such that the GCP extension knows the image URL for every version you want to offer.

An example `CloudProfileConfig` for the GCP extension looks as follows:

```yaml
apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
kind: CloudProfileConfig
machineImages:
- name: coreos
  versions:
  - version: 2135.6.0
    image: projects/coreos-cloud/global/images/coreos-stable-2135-6-0-v20190801
```

### Example `CloudProfile` manifest

If you want to allow that shoots can create VMs with local SSDs volumes then you have to specify the type of the disk with `SCRATCH` in the `.spec.volumeTypes[]` list.
Please find below an example `CloudProfile` manifest:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: CloudProfile
metadata:
  name: gcp
spec:
  type: gcp
  kubernetes:
    versions:
    - version: 1.16.1
    - version: 1.16.0
      expirationDate: "2020-04-05T01:02:03Z"
  machineImages:
  - name: coreos
    versions:
    - version: 2135.6.0
  machineTypes:
  - name: n1-standard-4
    cpu: "4"
    gpu: "0"
    memory: 15Gi
  volumeTypes:
  - name: pd-standard
    class: standard
  - name: pd-ssd
    class: premium
  - name: SCRATCH
    class: standard
  regions:
  - region: europe-west1
    names:
    - europe-west1-b
    - europe-west1-c
    - europe-west1-d
  providerConfig:
    apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
    kind: CloudProfileConfig
    machineImages:
    - name: coreos
      versions:
      - version: 2135.6.0
        image: projects/coreos-cloud/global/images/coreos-stable-2135-6-0-v20190801
```

## `Seed` resource

This provider extension does not support any provider configuration for the `Seed`'s `.spec.provider.providerConfig` field.
However, it supports to managing of backup infrastructure, i.e., you can specify a configuration for the `.spec.backup` field.

### Backup configuration

A Seed of type `gcp` can be configured to perform backups for the main etcds' of the shoot clusters control planes using Google Cloud Storage buckets.

The location/region where the backups will be stored defaults to the region of the Seed (`spec.provider.region`), but can also be explicitly configured via the field `spec.backup.region`.
The region of the backup can be different from where the seed cluster is running.
However, usually it makes sense to pick the same region for the backup bucket as used for the Seed cluster.

Please find below an example `Seed` manifest (partly) that configures backups using Google Cloud Storage buckets.

```yaml
---
apiVersion: core.gardener.cloud/v1beta1
kind: Seed
metadata:
  name: my-seed
spec:
  provider:
    type: gcp
    region: europe-west1
  backup:
    provider: gcp
    region: europe-west1 # default region
    secretRef:
      name: backup-credentials
      namespace: garden
  ...
```
An example of the referenced secret containing the credentials for the GCP Cloud storage can be found in the [example folder](https://github.com/gardener/gardener-extension-provider-gcp/blob/master/example/30-etcd-backup-secret.yaml).

#### Permissions for GCP Cloud Storage

Please make sure the service account associated with the provided credentials has the following IAM roles.
- [Storage Admin](https://cloud.google.com/storage/docs/access-control/iam-roles)

## Miscellaneous

### Gardener managed Service Accounts

The operators of the Gardener GCP extension can provide a list of managed service accounts (technical users) that can be used for GCP Shoots.
This eliminates the need for users to provide own service account for their clusters.

GCP service accounts are always bound to one project.
But there is an option to assign a service account originated in a different project of the same GCP organisation to a project.
Based on this approach Gardener operators can provide a project which contains managed service accounts and users could assign service accounts from this project with proper permissions to their projects.

To use this feature the user project need to have the organisation policy enabled that allow the assignment of service accounts originated in a different project of the same organisation.
More information are available [here](https://cloud.google.com/iam/docs/impersonating-service-accounts#binding-to-resources).

In case the user provide an own service account in the Shoot secret, this one will be used instead of the managed one provided by the operator.

Each managed service account will be maintained in a `Secret` like that:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: service-account-my-org
  namespace: extension-provider-gcp
  labels:
     gcp.provider.extensions.gardener.cloud/purpose: service-account-secret
data:
  orgID: base64(my-gcp-org-id)
  serviceaccount.json: base64(my-service-account-json-without-project-id-field)
type: Opaque
```

The user needs to provide in its Shoot secret a `orgID` and `projectID`.

The managed service account will be assigned based on the `orgID`.
In case there is a managed service account secret with a matching `orgID`, this one will be used for the Shoot.
If there is no matching managed service account secret then the next Shoot operation will fail.

One of the benefits of having managed service account is that the operator controls the lifecycle of the service account and can rotate its secrets.

After the service account secret has been rotated and the corresponding secret is updated, all Shoot clusters using it need to be reconciled or the last operation to be retried.
