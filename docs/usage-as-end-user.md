# Using the GCP provider extension with Gardener as end-user

The [`core.gardener.cloud/v1beta1.Shoot` resource](https://github.com/gardener/gardener/blob/master/example/90-shoot.yaml) declares a few fields that are meant to contain provider-specific configuration.

This document describes the configurable options for GCP and provides an example `Shoot` manifest with minimal configuration that can be used to create a GCP cluster (modulo the landscape-specific information like cloud profile names, secret binding names, etc.).

## GCP Provider Credentials

In order for Gardener to create a Kubernetes cluster using GCP infrastructure components, a Shoot has to provide credentials with sufficient permissions to the desired GCP project.
Every shoot cluster references a `SecretBinding` which itself references a `Secret`, and this `Secret` contains the provider credentials of the GCP project.
The `SecretBinding` is configurable in the [Shoot cluster](https://github.com/gardener/gardener/blob/master/example/90-shoot.yaml) with the field `secretBindingName`.

The required credentials for the GCP project are a [Service Account Key](https://cloud.google.com/iam/docs/service-accounts#service_account_keys) to authenticate as a [GCP Service Account](https://cloud.google.com/compute/docs/access/service-accounts).
A service account is a special account that can be used by services and applications to interact with Google Cloud Platform APIs. 
Applications can use service account credentials to authorize themselves to a set of APIs and perform actions within the permissions granted to the service account.

Make sure to [enable the Google Identity and Access Management (IAM) API](https://cloud.google.com/service-usage/docs/enable-disable).
[Create a Service Account](https://cloud.google.com/iam/docs/creating-managing-service-accounts) that shall be used for the Shoot cluster.
[Grant at least the following IAM roles](https://cloud.google.com/iam/docs/granting-changing-revoking-access) to the Service Account.
- Service Account Admin
- Service Account Token Creator
- Service Account User
- Compute Admin

Create a [JSON Service Account key](https://cloud.google.com/iam/docs/creating-managing-service-account-keys#creating_service_account_keys) for the Service Account.
Provide it in the `Secret` (base64 encoded for field `serviceaccount.json`), that is being referenced by the `SecretBinding` in the Shoot cluster configuration.

This `Secret` must look as follows:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: core-gcp
  namespace: garden-dev
type: Opaque
data:
  serviceaccount.json: base64(serviceaccount-json)
```

⚠️ Depending on your API usage it can be problematic to reuse the same Service Account Key for different Shoot clusters due to rate limits.
Please consider spreading your Shoots over multiple Service Accounts on different GCP projects if you are hitting those limits, see https://cloud.google.com/compute/docs/api-rate-limits.

## `InfrastructureConfig`

The infrastructure configuration mainly describes how the network layout looks like in order to create the shoot worker nodes in a later step, thus, prepares everything relevant to create VMs, load balancers, volumes, etc.

An example `InfrastructureConfig` for the GCP extension looks as follows:

```yaml
apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
kind: InfrastructureConfig
networks:
# vpc:
#   name: my-vpc
#   cloudRouter:
#     name: my-cloudrouter
  workers: 10.250.0.0/16
# internal: 10.251.0.0/16
# cloudNAT:
#   minPortsPerVM: 2048
#   natIPNames:
#   - name: manualnat1
#   - name: manualnat2
# flowLogs:
#   aggregationInterval: INTERVAL_5_SEC
#   flowSampling: 0.2
#   metadata: INCLUDE_ALL_METADATA
```

The `networks.vpc` section describes whether you want to create the shoot cluster in an already existing VPC or whether to create a new one:

* If `networks.vpc.name` is given then you have to specify the VPC name of the existing VPC that was created by other means (manually, other tooling, ...).
If you want to get a fresh VPC for the shoot then just omit the `networks.vpc` field.

* If a VPC name is not given then we will create the cloud router + NAT gateway to ensure that worker nodes don't get external IPs.

* If a VPC name is given then a cloud router name must also be given, failure to do so would result in validation errors
and possibly clusters without egress connectivity. 

* If a VPC name is given and calico shoot clusters are created without a network overlay within one VPC make sure that the pod CIDR specified in `shoot.spec.networking.pods` is not overlapping with any other pod CIDR used in that VPC.
Overlapping pod CIDRs will lead to disfunctional shoot clusters.

The `networks.workers` section describes the CIDR for a subnet that is used for all shoot worker nodes, i.e., VMs which later run your applications.

The `networks.internal` section is optional and can describe a CIDR for a subnet that is used for [internal load balancers](https://cloud.google.com/load-balancing/docs/internal/),

The `networks.cloudNAT.minPortsPerVM` is optional and is used to define the [minimum number of ports allocated to a VM for the CloudNAT](https://cloud.google.com/nat/docs/overview#number_of_nat_ports_and_connections)

The `networks.cloudNAT.natIPNames` is optional and is used to specify the names of the manual ip addresses which should be used by the nat gateway

The `networks.cloudNAT.endpointIndependentMapping` is optional and is used to define the [endpoint mapping behavior](https://cloud.google.com/nat/docs/ports-and-addresses#ports-reuse-endpoints).

The specified CIDR ranges must be contained in the VPC CIDR specified above, or the VPC CIDR of your already existing VPC.
You can freely choose these CIDRs and it is your responsibility to properly design the network layout to suit your needs.

The `networks.flowLogs` section describes the configuration for the VPC flow logs. In order to enable the VPC flow logs at least one of the following parameters needs to be specified in the flow log section:

* `networks.flowLogs.aggregationInterval` an optional parameter describing the aggregation interval for collecting flow logs. For more details, see [aggregation_interval reference](https://www.terraform.io/docs/providers/google/r/compute_subnetwork.html#aggregation_interval).

* `networks.flowLogs.flowSampling` an optional parameter describing the sampling rate of VPC flow logs within the subnetwork where 1.0 means all collected logs are reported and 0.0 means no logs are reported. For more details, see [flow_sampling reference](https://www.terraform.io/docs/providers/google/r/compute_subnetwork.html#flow_sampling).

* `networks.flowLogs.metadata` an optional parameter describing whether metadata fields should be added to the reported VPC flow logs. For more details, see [metadata reference](https://www.terraform.io/docs/providers/google/r/compute_subnetwork.html#metadata).

Apart from the VPC and the subnets the GCP extension will also create a dedicated service account for this shoot, and firewall rules.

## `ControlPlaneConfig`

The control plane configuration mainly contains values for the GCP-specific control plane components.
Today, the only component deployed by the GCP extension is the `cloud-controller-manager`.

An example `ControlPlaneConfig` for the GCP extension looks as follows:

```yaml
apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
kind: ControlPlaneConfig
zone: europe-west1-b
cloudControllerManager:
  featureGates:
    CustomResourceValidation: true
```

The `zone` field tells the cloud-controller-manager in which zone it should mainly operate.
You can still create clusters in multiple availability zones, however, the cloud-controller-manager requires one "main" zone.
:warning: You always have to specify this field!

The `cloudControllerManager.featureGates` contains a map of explicitly enabled or disabled feature gates.
For production usage it's not recommend to use this field at all as you can enable alpha features or disable beta/stable features, potentially impacting the cluster stability.
If you don't want to configure anything for the `cloudControllerManager` simply omit the key in the YAML specification.

## WorkerConfig

The worker configuration contains:

* Local SSD interface for the additional volumes attached to GCP worker machines.

  If you attach the disk with `SCRATCH` type, either an `NVMe` interface or a `SCSI` interface must be specified.
  It is only meaningful to provide this volume interface if only `SCRATCH` data volumes are used.
* Service Account with their specified scopes, authorized for this worker.

  Service accounts created in advance that generate access tokens that can be accessed through the metadata server and used to authenticate applications on the instance.

* GPU with its type and count per node. This will attach that GPU to all the machines in the worker grp

  **Note**: 
  * A rolling upgrade of the worker group would be triggered in case the `acceleratorType` or `count` is updated.
  * Some machineTypes like [a2 family](https://cloud.google.com/blog/products/compute/announcing-google-cloud-a2-vm-family-based-on-nvidia-a100-gpu) come with already attached gpu of `a100` type and pre-defined count. If your workerPool consists of those machineTypes, please **do not** specify any GPU configuration.
  * Sufficient quota of gpu is needed in the GCP project. This includes quota to support autoscaling if enabled.
  * GPU-attached machines can't be live migrated during host maintenance events. Find out how to handle that in your application [here](https://cloud.google.com/compute/docs/gpus/gpu-host-maintenance)
  * GPU count specified here is considered for forming node template during scale-from-zero in Cluster Autoscaler

  An example `WorkerConfig` for the GCP looks as follows:

```yaml
apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
kind: WorkerConfig
volume:
  interface: NVME
serviceAccount:
  email: foo@bar.com
  scopes:
  - https://www.googleapis.com/auth/cloud-platform
gpu:
  acceleratorType: nvidia-tesla-t4
  count: 1
```
## Example `Shoot` manifest

Please find below an example `Shoot` manifest:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: johndoe-gcp
  namespace: garden-dev
spec:
  cloudProfileName: gcp
  region: europe-west1
  secretBindingName: core-gcp
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
      zone: europe-west1-b
    workers:
    - name: worker-xoluy
      machine:
        type: n1-standard-4
      minimum: 2
      maximum: 2
      volume:
        size: 50Gi
        type: pd-standard
      zones:
      - europe-west1-b
  networking:
    nodes: 10.250.0.0/16
    type: calico
  kubernetes:
    version: 1.24.3
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

## CSI volume provisioners

Every GCP shoot cluster will be deployed with the GCP PD CSI driver.
It is compatible with the legacy in-tree volume provisioner that was deprecated by the Kubernetes community and will be removed in future versions of Kubernetes.
End-users might want to update their custom `StorageClass`es to the new `pd.csi.storage.gke.io` provisioner.

## Kubernetes Versions per Worker Pool

This extension supports `gardener/gardener`'s `WorkerPoolKubernetesVersion` feature gate, i.e., having [worker pools with overridden Kubernetes versions](https://github.com/gardener/gardener/blob/8a9c88866ec5fce59b5acf57d4227eeeb73669d7/example/90-shoot.yaml#L69-L70) since `gardener-extension-provider-gcp@v1.21`.

## Shoot CA Certificate and `ServiceAccount` Signing Key Rotation

This extension supports `gardener/gardener`'s `ShootCARotation` and `ShootSARotation` feature gates since `gardener-extension-provider-gcp@v1.23`.
