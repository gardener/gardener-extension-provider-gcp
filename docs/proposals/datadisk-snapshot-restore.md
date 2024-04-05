---
title: Data Disk Restore From Snapshot
creation-date: 2025-04-05
status: implementable
authors:
- "@elankath"
reviewers:
- "@rishabh-11"
- "@unmarshall"
- "@kon-angelo "
---

# Data Disk Restore From Snapshot

## Table of Contents

- [Summary](#summary)
- [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals](#non-goals)
- [Proposal](#proposal)
- [Alternatives](#alternatives)

## Summary

The GCP supports creation and restoration of disk snapshots. See [Disk Snapshots](https://cloud.google.com/compute/docs/disks/snapshots)

Currently, we have no support either in the shoot spec or in the [MCM GCP Provider](https://github.com/gardener/machine-controller-manager-provider-gcp) for restoring GCP Data Disks from snapshots

## Motivation
The primary motivation is to support [Integration of vSMP MemeoryOne in GCP](https://github.com/gardener/gardener-extension-provider-gcp/issues/695). 
Since we already implemented support for disk snapshot restoration in AWS via [Support for data volume snapshot ID ](https://github.com/gardener/gardener-extension-provider-aws/pull/112), we should introduce this enhancement
in GCP as well.

### Goals

1. Extend the GCP provider specific [WorkerConfig](https://github.com/gardener/gardener-extension-provider-gcp/blob/master/docs/usage/usage.md) section in the shoot YAML and support provider configuration for data-disks to support data-disk creation based from a snapshot name.
 

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
      snapshotName: snap-1234
  serviceAccount:
    email: foo@bar.com
    scopes:
      - https://www.googleapis.com/auth/cloud-platform
  gpu:
    acceleratorType: nvidia-tesla-t4
    count: 1
```

In the above `snap-1234` represents the snapshot name created by an external process/tool.
See [Create GCP Disk Snapshot](https://cloud.google.com/compute/docs/disks/create-snapshots#create_snapshots) as an example.

