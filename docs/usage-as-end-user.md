# Using the GCP provider extension with Gardener as end-user

The [`core.gardener.cloud/v1beta1.Shoot` resource](https://github.com/gardener/gardener/blob/master/example/90-shoot.yaml) declares a few fields that are meant to contain provider-specific configuration.

In this document we are describing how this configuration looks like for GCP and provide an example `Shoot` manifest with minimal configuration that you can use to create an GCP cluster (modulo the landscape-specific information like cloud profile names, secret binding names, etc.).

## Provider Secret Data

Every shoot cluster references a `SecretBinding` which itself references a `Secret`, and this `Secret` contains the provider credentials of your GCP project.
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

Please look up https://cloud.google.com/iam/docs/creating-managing-service-accounts as well.

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
#   aggregationInterval: 0.5
#   flowSampling: 0.2
#   metadata: INCLUDE_ALL_METADATA
```

The `networks.vpc` section describes whether you want to create the shoot cluster in an already existing VPC or whether to create a new one:

* If `networks.vpc.name` is given then you have to specify the VPC name of the existing VPC that was created by other means (manually, other tooling, ...).
If you want to get a fresh VPC for the shoot then just omit the `networks.vpc` field.

* If a VPC name is not given then we will create the cloud router + NAT gateway to ensure that worker nodes don't get external IPs.

* If a VPC name is given then a cloud router name must also be given, failure to do so would result in validation errors
and possibly clusters without egress connectivity.

The `networks.workers` section describes the CIDR for a subnet that is used for all shoot worker nodes, i.e., VMs which later run your applications.

The `networks.internal` section is optional and can describe a CIDR for a subnet that is used for [internal load balancers](https://cloud.google.com/load-balancing/docs/internal/),

The `networks.cloudNAT.minPortsPerVM` is optional and is used to define the [minimum number of ports allocated to a VM for the CloudNAT](https://cloud.google.com/nat/docs/overview#number_of_nat_ports_and_connections)

The `networks.cloudNAT.natIPNames` is optional and is used to specify the names of the manual ip addresses which should be used by the nat gateway

The specified CIDR ranges must be contained in the VPC CIDR specified above, or the VPC CIDR of your already existing VPC.
You can freely choose these CIDRs and it is your responsibility to properly design the network layout to suit your needs.

The `networks.flowLogs` section describes the configuration for the VPC flow logs. In order to enable the VPC flow logs at least one of the following parameters needs to be specified in the flow log section:

* `networks.flowLogs.aggregationInterval` an optional parameter describing the aggregation interval for collecting flow logs.

* `networks.flowLogs.flowSampling` an optional parameter describing the sampling rate of VPC flow logs within the subnetwork where 1.0 means all collected logs are reported and 0.0 means no logs are reported.

* `networks.flowLogs.metadata` an optional parameter describing whether metadata fields should be added to the reported VPC flow logs.

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

The worker configuration mainly contains Local SSD interface for the additional volumes attached to GCP worker machines.
If you attach the disk with `SCRATCH` type, either an `NVMe` interface or a `SCSI` interface must be specified.
It is only meaningful to provide this volume interface if only `SCRATCH` data volumes are used.
An example `WorkerConfig` for the GCP looks as follows:

```yaml
apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
kind: WorkerConfig
volume:
  interface: NVME
```

## Example `Shoot` manifest

Please find below an example `Shoot` manifest:

```yaml
apiVersion: core.gardener.cloud/v1alpha1
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
    version: 1.16.1
  maintenance:
    autoUpdate:
      kubernetesVersion: true
      machineImageVersion: true
  addons:
    kubernetes-dashboard:
      enabled: true
    nginx-ingress:
      enabled: true
```

## CSI volume provisioners

Every GCP shoot cluster that has at least Kubernetes v1.18 will be deployed with the GCP PD CSI driver.
It is compatible with the legacy in-tree volume provisioner that was deprecated by the Kubernetes community and will be removed in future versions of Kubernetes.
End-users might want to update their custom `StorageClass`es to the new `pd.csi.storage.gke.io` provisioner.
Shoot clusters with Kubernetes v1.17 or less will use the in-tree `kubernetes.io/gce-pd` volume provisioner in the kube-controller-manager and the kubelet.


## ip-masq-agent

GCP blocks non-VM egress traffic by default in their infrastructures. As a result traffic to the outside needs 
to be masqueraded to the node ip when egressing. The [ip-masq-agent](https://github.com/kubernetes-sigs/ip-masq-agent)  does 
exactly that. 

The `ip-masq-agent` [daemonset](charts/internal/shoot-system-components/charts/ip-masq-agent/templates/daemonset.yaml) mounts an optional configmap that can be configured post cluster creation. 

```yaml
volumes:
# this configmap is optional, i.e., it does not have to exist at creation time. It can however be created
# to reconfigure the default behavior of the ip-masq-agent on a live-cluster.
- configMap:
    defaultMode: 420
    items:
      - key: config
        path: ip-masq-agent
    name: ip-masq-agent
    optional: true
  name: config
```

```yaml
apiVersion: v1
data:
  config: |-
    nonMasqueradeCIDRs:
      - 10.0.0.0/8
      - 172.16.0.0/12
      - 192.168.0.0/16
    masqLinkLocal: false
    masqLinkLocalIPv6: false
    resyncInterval: 60s
kind: ConfigMap
metadata:
  name: ip-masq-agent
```

The alternative to using the `ip-masq-agent` would be `Alias IPs`, however, `Alias IPs` requires additional configuration 
on the machine level and manual IP address management and assignment for the secondary networks (the pod network).