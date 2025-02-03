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
For each machine image version an `architecture` field can be specified which specifies the CPU architecture of the machine on which given machine image can be used.

An example `CloudProfileConfig` for the GCP extension looks as follows:

```yaml
apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
kind: CloudProfileConfig
machineImages:
- name: coreos
  versions:
  - version: 2135.6.0
    image: projects/coreos-cloud/global/images/coreos-stable-2135-6-0-v20190801
    # architecture: amd64 # optional
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
    - version: 1.27.3
    - version: 1.26.8
      expirationDate: "2022-10-31T23:59:59Z"
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
        # architecture: amd64 # optional
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
An example of the referenced secret containing the credentials for the GCP Cloud storage can be found in the [example folder](../../example/30-etcd-backup-secret.yaml).

#### Permissions for GCP Cloud Storage

Please make sure the service account associated with the provided credentials has the following IAM roles.
- [Storage Admin](https://cloud.google.com/storage/docs/access-control/iam-roles)

### In-place vs Rolling-updates of Shoot Workers

Changes to the `Shoot` worker-pools are applied in-place where possible. In case this is not possible a rolling update of the workers will be performed to apply the new configuration, as outlined in [the Gardener documentation](https://github.com/gardener/gardener/blob/master/docs/usage/shoot-operations/shoot_updates.md#in-place-vs-rolling-updates). The exact fields that trigger this behaviour depend on whether the feature gate `NewWorkerPoolHash` is enabled. If it is not enabled, only the fields mentioned in the [Gardener doc](https://github.com/gardener/gardener/blob/master/docs/usage/shoot-operations/shoot_updates.md#rolling-update-triggers) are used.
If the feature gate _is_ enabled, instead of the complete provider config only the following fields are used:

- `.spec.provider.workers[].dataVolumes[].name`
- `.spec.provider.workers[].dataVolumes[].size`
- `.spec.provider.workers[].dataVolumes[].type`
- `.spec.provider.workers[].dataVolumes[].encrypted`
- `.spec.provider.workers[].providerConfig.volume.encryption`
- `.spec.provider.workers[].providerConfig.volume.localSsdInterface`
- `.spec.provider.workers[].providerConfig.dataVolumes[].name`
- `.spec.provider.workers[].providerConfig.dataVolumes[].sourceImage`
- `.spec.provider.workers[].providerConfig.dataVolumes[].provisionedIops`
- `.spec.provider.workers[].providerConfig.minCpuPlatform`
- `.spec.provider.workers[].providerConfig.gpu`
- `.spec.provider.workers[].providerConfig.serviceAccount`
