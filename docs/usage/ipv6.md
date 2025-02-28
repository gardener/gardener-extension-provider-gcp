# DualStack Support for Gardener GCP Extension

This document provides an overview of DualStack support for the Gardener GCP extension, detailing its functionality, requirements, and implementation specifics. The document also clarifies the differences between provisioning methods and the unique components required for DualStack clusters.

---

## Overview

DualStack support allows Gardener GCP shoot clusters to leverage both IPv4 and IPv6 addresses. It is supported exclusively when using the **InfraFlow Controller**. The legacy Terraform reconciler does not support DualStack provisioning.

### Key Features

- **DualStack Subnets**: Separate subnets are created for nodes and services, with explicit IPv4 and IPv6 ranges.
- **Ingress-GCE Component**: Responsible for creating IPv6 Load Balancers.
- **Cloud Allocator for IPAM**: Manages the assignment of IPv4 and IPv6 ranges to nodes and pods.

---

## Provisioning Options

### 1. Terraform Reconciler

- Legacy approach.
- Does **not support DualStack provisioning**.

### 2. InfraFlow Controller

- Supports DualStack clusters.
- Requires the annotation `provider.extensions.gardener.cloud/use-flow: "true"` to be added to the shoot object.
- This annotation is **mandatory** for:
  - Creating a new DualStack shoot.
  - Migrating an existing IPv4-only shoot to DualStack.

---

## Subnet Configuration for DualStack

When provisioning a DualStack cluster, the GCP provider creates distinct subnets:

### 1. **Node Subnet**

- **Primary IPv4 Range**: Used for IPv4 nodes.
- **Secondary IPv4 Range**: Used for IPv4 pods.
- **External IPv6 Range**: Auto-assigned with a `/64` prefix. Each VM gets an interface with a `/96` prefix.
- **Customization**:
  - IPv4 ranges (pods and nodes) can be defined in the shoot object.
  - IPv6 ranges are automatically filled by the GCP provider.

### 2. **Service Subnet**

- This subnet is dedicated to IPv6 services. It is created due to GCP's limitation of not supporting IPv6 reservation for services.

---

## Additional Components

### 1. **Ingress-GCE**

- The ingress-gce is a mandatory component for DualStack clusters. It is responsible for creating IPv6 Load Balancers. This is necessary because the GCP Cloud Controller Manager (CCM) does not support IPv6 Load Balancer creation.

---

## Cloud Allocator (IPAM)

The Cloud Allocator is part of the GCP Cloud Controller Manager (CCM) and plays a critical role in managing IPAM (IP Address Management) for DualStack clusters. 

### Responsibilities

- **Assigning PODCIDRs to Node Objects**: Ensures that both IPv4 and IPv6 pod ranges are correctly assigned to the node objects.
- **Leveraging Secondary IPv4 Range**:
  - Uses the secondary IPv4 range in the node subnet to allocate pod IP ranges.
  - Assigns both IPv4 and IPv6 pod ranges in compliance with GCP’s networking model.

### Operational Details

- The Cloud Allocator uses a `/119` prefix from the external IPv6 range assigned to each VM.
- This ensures efficient utilization of IPv6 address space while maintaining compatibility with Kubernetes networking requirements.

#### Why Use a Secondary IPv4 Range for Pods?
The secondary IPv4 range is essential for:
- Enabling the Cloud Allocator to function correctly in assigning IP ranges.
- Supporting both IPv4 and IPv6 pods in DualStack clusters.
- Aligning with GCP CCM’s requirement to separate pod IP ranges within the node subnet.

---

## Creating a DualStack Cluster

To create a DualStack cluster, rely on the `spec.networking.ipFamilies` field to specify the desired stack. Below is an example of a DualStack shoot configuration:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
 annotations:
   provider.extensions.gardener.cloud/use-flow: "true"
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

### Explanation
- **`spec.networking.ipFamilies`**: Specifies the stack (IPv4, IPv6, or both). In this example, both IPv4 and IPv6 are defined for a DualStack cluster.
- **Annotation**: `provider.extensions.gardener.cloud/use-flow: "true"` is mandatory for DualStack support.

---

## Migration of IPv4-only Shoot Clusters to Dual-Stack

Eventually, migration should be as easy as changing the `.spec.networking.ipFamilies` field in the `Shoot` resource from `IPv4` to `IPv4, IPv6`.
However, as of now, this is not supported.

It is worth recognizing that the migration from an IPv4-only shoot cluster to a dual-stack shoot cluster involves rolling of the nodes/workload as well.
Nodes will not get a new IPv6 address assigned automatically.
The same is true for pods as well.
Once the migration is supported, the detailed caveats will be documented here.

---

## Load Balancer Service Configuration

When creating LoadBalancer services in a DualStack cluster, you must include specific annotations to enable proper reconciliation by the `ingress-gce` component for IPv6 support.

### Required Annotation for IPv6 LoadBalancers
Add the following annotation to the service:
```yaml
cloud.google.com/l4-rbs: enabled
```

### Internal Load Balancer Considerations
- Internal IPv6 LoadBalancers are **not supported**.
- For internal IPv4 LoadBalancers, you can use:
  - `"networking.gke.io/load-balancer-type=Internal"`
  - `"cloud.google.com/load-balancer-type=internal"` (deprecated).
  They are created by cloud-controller-manger and get an an IPv4 address from the internal subnet.

### Example Configuration
Here is an example of a DualStack LoadBalancer service configuration:

```yaml
apiVersion: v1
kind: Service
metadata:
  annotations:
    cloud.google.com/l4-rbs: enabled
  name: webapp2
  namespace: default
spec:
  ipFamilyPolicy: RequireDualStack
  ipFamilies:
  - IPv4
  - IPv6
  ports:
  - port: 80
    targetPort: 80
    protocol: TCP
  selector:
    run: webapp2
  type: LoadBalancer
```

### Explanation
- **`cloud.google.com/l4-rbs: enabled`**: Ensures that `ingress-gce` properly reconciles the LoadBalancer service with IPv6 support.
- **`ipFamilyPolicy` and `ipFamilies`**: Specify the DualStack configuration.
- **Internal LoadBalancer**: Use the specific annotations for internal IPv4-only LoadBalancers.
---

## Key Benefits of DualStack Support

1. **Improved Network Compatibility**: DualStack enables seamless communication across both IPv4 and IPv6 environments.
2. **Enhanced Scalability**: IPv6 significantly expands the available address space.
3. **Future-Proofing**: DualStack readiness ensures compliance with modern networking standards.

---

## Summary

- DualStack is supported only with the **InfraFlow Controller**.
- The annotation `provider.extensions.gardener.cloud/use-flow: "true"` is mandatory for enabling DualStack.
- Dedicated subnets for nodes and services are created to manage IPv4 and IPv6 ranges.
- Components like **Ingress-GCE** and the **Cloud Allocator** ensure proper functionality and Load Balancer creation.
- Existing IPv4 clusters must have been created with InfraFlow or been migrated to it to be eligible for dual-stack migration once available.

DualStack support in Gardener GCP extension represents a significant advancement in networking capabilities, catering to modern cloud-native requirements.





