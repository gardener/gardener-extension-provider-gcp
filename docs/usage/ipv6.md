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

- This subnet is dedicated to IPv6 services. it is created due to GCP's limitation of not supporting IPv6 reservation for services.

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
    confirmation.gardener.cloud/deletion: "true"
  name: gcp-shoot-1
spec:
  cloudProfileName: gcp
  region: europe-north1
  secretBindingName: infra-credentials
  kubernetes:
    version: 1.31.2
  provider:
    type: gcp
    infrastructureConfig:
      apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
      kind: InfrastructureConfig
      networks:
        workers: 10.250.0.0/16
    controlPlaneConfig:
      apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
      kind: ControlPlaneConfig
      zone: europe-north1-a
    workers:
    - name: worker-med
      machine:
        type: n1-standard-4
      minimum: 2
      maximum: 2
      zones:
        - europe-north1-a
        - europe-north1-b
      volume:
        size: 50Gi
        type: pd-standard
  networking:
    ipFamilies:
    - IPv4
    - IPv6
    type: calico
    services: 10.253.0.0/16
    nodes: 10.250.0.0/16
    pods: 10.255.0.0/16
    providerConfig:
      apiVersion: calico.networking.extensions.gardener.cloud/v1alpha1
      kind: NetworkConfig
      overlay:
        enabled: false
  maintenance:
    autoUpdate:
      kubernetesVersion: true
      machineImageVersion: true
  addons:
    kubernetesDashboard:
      enabled: true
    nginxIngress:
      enabled: true
```

### Explanation
- **`spec.networking.ipFamilies`**: Specifies the stack (IPv4, IPv6, or both). In this example, both IPv4 and IPv6 are defined for a DualStack cluster.
- **Annotation**: `provider.extensions.gardener.cloud/use-flow: "true"` is mandatory for DualStack support.

---

## Migrating a SingleStack Cluster to DualStack

To migrate an existing IPv4-only shoot cluster to DualStack, follow these steps:

1. **Ensure InfraFlow was Used**:
   - The existing IPv4 cluster must have been created using the InfraFlow Controller. Migration is not supported for clusters created with the Terraform reconciler.

2. **Edit the Shoot Specification**:
   - Update the `spec.networking.ipFamilies` field to include both `IPv4` and `IPv6`.
   - Add the annotation `gardener.cloud/operation: "maintain"` to the shoot object to trigger reconciliation across the necessary controllers.

3. **Wait for Reconciliation**:
   - The shoot cluster must be fully reconciled before the migration is complete. Ensure that the cluster is 100% reconciled before proceeding.

4. **Migration Process**:
   - The migration process involves the following steps:
     - Scale down the GCP Cloud Controller Manager (CCM).
     - Clean up existing routes.
     - Update the subnets:
       - Add secondary ranges.
       - Change stack type and IPv6 access type.
     - Add the service subnet for IPv6.
     - **Update and add IPv6 firewall rules** after creating the subnets.
     - Deploy the Ingress-GCE component.
     - Update managed resources in the control plane (e.g., CCM, kube-apiserver, kube-controller-manager).
     - Generate new machine classes and machine deployments.
     - Roll out the worker nodes and create new machines with both IPv4 and IPv6 interfaces.

5. **Verify the Migration**:
   - Once the migration is complete, verify that all resources, including pods, services, and worker nodes, are correctly configured with DualStack support.

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
- Migrating a SingleStack cluster to DualStack requires editing the shoot specification and waiting for complete reconciliation.
- Existing IPv4 clusters must have been created with InfraFlow to be eligible for migration.

DualStack support in Gardener GCP extension represents a significant advancement in networking capabilities, catering to modern cloud-native requirements.





