---
title: Data Disk Restore From Image
creation-date: 2025-04-05
status: implementable
authors:
- "@elankath"
reviewers:
- "@rishabh-11"
- "@unmarshall"
- "@kon-angelo "
---

# Data Disk Restore From Image

## Table of Contents

- [Summary](#summary)
- [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals](#non-goals)
- [Proposal](#proposal)
- [Alternatives](#alternatives)

## Summary

Currently, we have no support either in the shoot spec or in the [MCM GCP Provider](https://github.com/gardener/machine-controller-manager-provider-gcp) for restoring GCP Data Disks from images. 

## Motivation
The primary motivation is to support [Integration of vSMP MemeoryOne in Azure](https://github.com/gardener/gardener-extension-provider-azure/issues/788).
We implemented support for this in AWS via [Support for data volume snapshot ID ](https://github.com/gardener/gardener-extension-provider-aws/pull/112).
In GCP we have the option to restore data disk from a custom image which is more convenient and flexible.

### Goals

1. Extend the GCP provider specific [WorkerConfig](https://github.com/gardener/gardener-extension-provider-gcp/blob/master/docs/usage/usage.md) section in the shoot YAML and support provider configuration for
 data-disks to support data-disk creation from an image name by supplying an image name.
 

## Proposal

### Shoot Specification

At this current time, there is no support for provider specific configuration of data disks in an GCP shoot spec.
The below shows an example configuration at the time of this proposal:
```yaml
providerConfig:
  apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
  kind: WorkerConfig
  volume:
    interface: NVME
    encryption: # optional, skipped detail here
  serviceAccount:
    email: foo@bar.com
    scopes:
      - https://www.googleapis.com/auth/cloud-platform
  gpu:
    acceleratorType: nvidia-tesla-t4
    count: 1
```
We propose that the worker config section be enahnced to support data disk configuration
```yaml
providerConfig:
  apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
  kind: WorkerConfig
  volume:
    interface: NVME
    encryption: # optional, skipped detail here
  dataVolumes: # <-- NEW SUB_SECTION
    - name: vsmp1
      imageName: imgName
  serviceAccount:
    email: foo@bar.com
    scopes:
      - https://www.googleapis.com/auth/cloud-platform
  gpu:
    acceleratorType: nvidia-tesla-t4
    count: 1
```

In the above `imgName` represents the image name of a previously created image created by a tool or process.
See [Google Cloud Create Image](https://cloud.google.com/sdk/gcloud/reference/compute/images/create).

The [MCM GCP Provider](https://github.com/gardener/machine-controller-manager-provider-gcp) will ensure when a VM instance is instantiated, that the data
disk(s) for the VM are created with the _source image_ set to the provided `imgName`. 
The mechanics of this is left to MCM GCP provider. See `image` param to `--create-disk` flag in
[Google Cloud Instance Creation](https://cloud.google.com/sdk/gcloud/reference/compute/instances/create)




