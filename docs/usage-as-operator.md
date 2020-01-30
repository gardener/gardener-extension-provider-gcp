# Using the GCP provider extension with Gardener as operator

The [`core.gardener.cloud/v1alpha1.CloudProfile` resource](https://github.com/gardener/gardener/blob/master/example/30-cloudprofile.yaml) declares a `providerConfig` field that is meant to contain provider-specific configuration.

In this document we are describing how this configuration looks like for GCP and provide an example `CloudProfile` manifest with minimal configuration that you can use to allow creating GCP shoot clusters.

## `CloudProfileConfig`

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

## Example `CloudProfile` manifest

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
