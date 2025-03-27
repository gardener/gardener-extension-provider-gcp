# Dual-stack Support for Gardener GCP Extension

This document provides an overview of dual-stack support for the Gardener GCP extension.
Furthermore it clarifies which components are utilized, how the infrastructure is setup and how a dual-stack cluster can be provisioned.

## Overview

Gardener allows to create dual-stack shoot clusters on GCP. In this mode, both IPv4 and IPv6 are supported within the shoot cluster.
This significantly expands the available address space, enables seamless communication across both IPv4 and IPv6 environments and ensures compliance with modern networking standards.

### Key Components for Dual-Stack Support

- **Dual-stack Subnets**: Separate subnets are created for nodes and services, with explicit IPv4 and external IPv6 ranges.
- **Ingress-GCE Component**: Responsible for creating dual-stack (IPv4,IPv6) Load Balancers.
- **Cloud Allocator for IPAM**: Manages the assignment of IPv4 and IPv6 ranges to nodes and pods.

## Subnet Configuration for dual-stack

When provisioning a dual-stack cluster, the GCP provider creates distinct subnets:

### 1. **Node Subnet**

- **Primary IPv4 Range**: Used for IPv4 nodes.
- **Secondary IPv4 Range**: Used for IPv4 pods.
- **External IPv6 Range**: Auto-assigned with a `/64` prefix. Each VM gets an interface with a `/96` prefix.
- **Customization**:
  - IPv4 ranges (pods and nodes) can be defined in the shoot object.
  - IPv6 ranges are automatically filled by the GCP provider.

### 2. **Service Subnet**

- This subnet is dedicated to IPv6 services. It is created due to GCP's limitation of not supporting IPv6 range reservations.

### 3. **Internal Subnet (optional)**

- This subnet is dedicated for internal load balancer. Currently, only internal IPv4 loadbalancer are supported. They are provisioned by the Cloud Controller Manager (CCM).

## **Ingress-GCE**

- The ingress-gce is a mandatory component for dual-stack clusters. It is responsible for creating dual-stack (IPv4,IPv6) Load Balancers. This is necessary because the GCP Cloud Controller Manager does not support provisioning IPv6 Load Balancer.


## Cloud Allocator (IPAM)

The Cloud Allocator is part of the GCP Cloud Controller Manager and plays a critical role in managing IPAM (IP Address Management) for dual-stack clusters. 

### Responsibilities

- **Assigning PODCIDRs to Node Objects**: Ensures that both IPv4 and IPv6 pod ranges are correctly assigned to the node objects.
- **Leveraging Secondary IPv4 Range**:
  - Uses the secondary IPv4 range in the node subnet to allocate pod IP ranges.
  - Assigns both IPv4 and IPv6 pod ranges in compliance with GCP’s networking model.

### Operational Details

- The Cloud Allocator assigns a `/112` pod cidr range/subrange from the `/96` cidr range assigned to each VM.
- This ensures efficient utilization of IPv6 address space while maintaining compatibility with Kubernetes networking requirements.

#### Why Use a Secondary IPv4 Range for Pods?
The secondary IPv4 range is essential for:
- Enabling the Cloud Allocator to function correctly in assigning IP ranges.
- Supporting both IPv4 and IPv6 pods in dual-stack clusters.
- Aligning with GCP CCM’s requirement to separate pod IP ranges within the node subnet.


## Creating a dual-stack Cluster

To create a dual-stack cluster, both IP families (IPv4,IPv6) need to be specified under `spec.networking.ipFamilies`. Below is an example of a dual-stack shoot cluster configuration:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  ...
spec:
  ...
  provider:
    type: gcp
    infrastructureConfig:
      apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
      kind: InfrastructureConfig
      networks:
        workers: 10.250.0.0/16
  ...
  networking:
    type: ...
    ipFamilies:
    - IPv4
    - IPv6
    nodes: 10.250.0.0/16
  ...
```

## Migration of IPv4-only Shoot Clusters to dual-stack

To trigger a migration of an IPv4 shoot cluster to DualStack, IPv6 needs to be added to `spec.networking.ipFamilies` in the shoot specification.
Once the migration is triggered a constraint of type `ToDualStackMigration` is added to the shoot status. It is in progressing state.
With the next shoot maintenance, the infrastructure is migrated to IPv6. The subnets get an external IPv6 range, the node subnet gets the secondary IPv4 range. The pod specific cloud routes are deleted from the VPC route table and alias IP ranges for the pod routes are added to the NIC of kubernetes nodes/instances. After that the status of the constraint will be changed to `DualStackInfraReady`. 
With the next node roll-out, nodes will get IPv6 addresses and an IPv6 prefix for pods. When all nodes have IPv4 and IPv6 pod ranges, the status will be changed to `DualStackNodesReady`.
The next reconcile will change all the remaining components to dual-stack. Once the migration has finished the constraint will be removed with the next reconcile.

## Load Balancer Service Configuration

To create a dual-stack LoadBalancer the `spec.ipFamilies` and `spec.ipFamilyPolicy` field needs to be specified in the kubernetes service.
An example configuration is shown below:

```
apiVersion: v1
kind: Service
metadata:
  name: nginx
  namespace: default
  annotations:
    cloud.google.com/l4-rbs: enabled
spec:
  ipFamilies:
  - IPv4
  - IPv6
  ipFamilyPolicy: PreferDualStack
  ports:
  - port: 12345
    protocol: TCP
    targetPort: 80
  selector:
    run: nginx
  type: LoadBalancer
```

The required annotation `cloud.google.com/l4-rbs: enabled` for ingress-gce is added automatically via webhook for services of `type: LoadBalancer`.

### Internal Load Balancer
- Internal IPv6 LoadBalancers are currently **not supported**.
- To create internal IPv4 LoadBalancers, you can set one of the the following annotations:
  - `"networking.gke.io/load-balancer-type=Internal"`
  - `"cloud.google.com/load-balancer-type=internal"` (deprecated).
  Internal load balancers are created by cloud-controller-manger and get an IPv4 address from the internal subnet.
