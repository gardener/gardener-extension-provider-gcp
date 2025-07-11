<p>Packages:</p>
<ul>
<li>
<a href="#gcp.provider.extensions.gardener.cloud%2fv1alpha1">gcp.provider.extensions.gardener.cloud/v1alpha1</a>
</li>
</ul>
<h2 id="gcp.provider.extensions.gardener.cloud/v1alpha1">gcp.provider.extensions.gardener.cloud/v1alpha1</h2>
<p>
<p>Package v1alpha1 contains the GCP provider API resources.</p>
</p>
Resource Types:
<ul><li>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.BackupBucketConfig">BackupBucketConfig</a>
</li><li>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.CloudProfileConfig">CloudProfileConfig</a>
</li><li>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.ControlPlaneConfig">ControlPlaneConfig</a>
</li><li>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.InfrastructureConfig">InfrastructureConfig</a>
</li><li>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.WorkerConfig">WorkerConfig</a>
</li><li>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.WorkloadIdentityConfig">WorkloadIdentityConfig</a>
</li></ul>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.BackupBucketConfig">BackupBucketConfig
</h3>
<p>
<p>BackupBucketConfig represents the configuration for a backup bucket.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code></br>
string</td>
<td>
<code>
gcp.provider.extensions.gardener.cloud/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>BackupBucketConfig</code></td>
</tr>
<tr>
<td>
<code>immutability</code></br>
<em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.ImmutableConfig">
ImmutableConfig
</a>
</em>
</td>
<td>
<p>Immutability defines the immutability config for the backup bucket.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.CloudProfileConfig">CloudProfileConfig
</h3>
<p>
<p>CloudProfileConfig contains provider-specific configuration that is embedded into Gardener&rsquo;s <code>CloudProfile</code>
resource.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code></br>
string</td>
<td>
<code>
gcp.provider.extensions.gardener.cloud/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>CloudProfileConfig</code></td>
</tr>
<tr>
<td>
<code>machineImages</code></br>
<em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.MachineImages">
[]MachineImages
</a>
</em>
</td>
<td>
<p>MachineImages is the list of machine images that are understood by the controller. It maps
logical names and versions to provider-specific identifiers.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.ControlPlaneConfig">ControlPlaneConfig
</h3>
<p>
<p>ControlPlaneConfig contains configuration settings for the control plane.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code></br>
string</td>
<td>
<code>
gcp.provider.extensions.gardener.cloud/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>ControlPlaneConfig</code></td>
</tr>
<tr>
<td>
<code>zone</code></br>
<em>
string
</em>
</td>
<td>
<p>Zone is the GCP zone.</p>
</td>
</tr>
<tr>
<td>
<code>cloudControllerManager</code></br>
<em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.CloudControllerManagerConfig">
CloudControllerManagerConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CloudControllerManager contains configuration settings for the cloud-controller-manager.</p>
</td>
</tr>
<tr>
<td>
<code>storage</code></br>
<em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.Storage">
Storage
</a>
</em>
</td>
<td>
<p>Storage contains configuration for the storage in the cluster.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.InfrastructureConfig">InfrastructureConfig
</h3>
<p>
<p>InfrastructureConfig infrastructure configuration resource</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code></br>
string</td>
<td>
<code>
gcp.provider.extensions.gardener.cloud/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>InfrastructureConfig</code></td>
</tr>
<tr>
<td>
<code>networks</code></br>
<em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.NetworkConfig">
NetworkConfig
</a>
</em>
</td>
<td>
<p>Networks is the network configuration (VPC, subnets, etc.)</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.WorkerConfig">WorkerConfig
</h3>
<p>
<p>WorkerConfig contains configuration settings for the worker nodes.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code></br>
string</td>
<td>
<code>
gcp.provider.extensions.gardener.cloud/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>WorkerConfig</code></td>
</tr>
<tr>
<td>
<code>gpu</code></br>
<em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.GPU">
GPU
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>GPU contains configuration for the GPU attached to VMs.</p>
</td>
</tr>
<tr>
<td>
<code>volume</code></br>
<em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.Volume">
Volume
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Volume contains configuration for the root disks attached to VMs.</p>
</td>
</tr>
<tr>
<td>
<code>dataVolumes</code></br>
<em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.DataVolume">
[]DataVolume
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DataVolumes contains configuration for the additional disks attached to VMs.</p>
</td>
</tr>
<tr>
<td>
<code>minCpuPlatform</code></br>
<em>
string
</em>
</td>
<td>
<p>MinCpuPlatform is the name of the minimum CPU platform that is to be
requested for the VM.</p>
</td>
</tr>
<tr>
<td>
<code>serviceAccount</code></br>
<em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.ServiceAccount">
ServiceAccount
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Service account, with their specified scopes, authorized for this worker.
Service accounts generate access tokens that can be accessed through
the metadata server and used to authenticate applications on the
instance.
This service account should be created in advance.</p>
</td>
</tr>
<tr>
<td>
<code>nodeTemplate</code></br>
<em>
github.com/gardener/gardener/pkg/apis/extensions/v1alpha1.NodeTemplate
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeTemplate contains resource information of the machine which is used by Cluster Autoscaler to generate nodeTemplate during scaling a nodeGroup from zero.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.WorkloadIdentityConfig">WorkloadIdentityConfig
</h3>
<p>
<p>WorkloadIdentityConfig contains configuration settings for workload identity.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code></br>
string</td>
<td>
<code>
gcp.provider.extensions.gardener.cloud/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>WorkloadIdentityConfig</code></td>
</tr>
<tr>
<td>
<code>projectID</code></br>
<em>
string
</em>
</td>
<td>
<p>ProjectID is the ID of the GCP project.</p>
</td>
</tr>
<tr>
<td>
<code>credentialsConfig</code></br>
<em>
k8s.io/apimachinery/pkg/runtime.RawExtension
</em>
</td>
<td>
<p>CredentialsConfig contains information for workload authentication against GCP.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.CSIFilestore">CSIFilestore
</h3>
<p>
(<em>Appears on:</em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.Storage">Storage</a>)
</p>
<p>
<p>CSIFilestore contains configuration for CSI Filestore driver</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>enabled</code></br>
<em>
bool
</em>
</td>
<td>
<p>Enabled is the switch to enable the CSI Manila driver support</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.CloudControllerManagerConfig">CloudControllerManagerConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.ControlPlaneConfig">ControlPlaneConfig</a>)
</p>
<p>
<p>CloudControllerManagerConfig contains configuration settings for the cloud-controller-manager.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>featureGates</code></br>
<em>
map[string]bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>FeatureGates contains information about enabled feature gates.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.CloudNAT">CloudNAT
</h3>
<p>
(<em>Appears on:</em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.NetworkConfig">NetworkConfig</a>)
</p>
<p>
<p>CloudNAT contains configuration about the CloudNAT resource</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>endpointIndependentMapping</code></br>
<em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.EndpointIndependentMapping">
EndpointIndependentMapping
</a>
</em>
</td>
<td>
<p>EndpointIndependentMapping controls if endpoint independent mapping is enabled.</p>
</td>
</tr>
<tr>
<td>
<code>minPortsPerVM</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>MinPortsPerVM is the minimum number of ports allocated to a VM in the NAT config.
The default value is 2048 ports.</p>
</td>
</tr>
<tr>
<td>
<code>maxPortsPerVM</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxPortsPerVM is the maximum number of ports allocated to a VM in the NAT config.
The default value is 65536 ports.</p>
</td>
</tr>
<tr>
<td>
<code>enableDynamicPortAllocation</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>EnableDynamicPortAllocation controls port allocation behavior for the CloudNAT.</p>
</td>
</tr>
<tr>
<td>
<code>natIPNames</code></br>
<em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.NatIPName">
[]NatIPName
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NatIPNames is a list of all user provided external premium ips which can be used by the nat gateway</p>
</td>
</tr>
<tr>
<td>
<code>icmpIdleTimeoutSec</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>IcmpIdleTimeoutSec is the timeout (in seconds) for ICMP connections. Defaults to 30.</p>
</td>
</tr>
<tr>
<td>
<code>tcpEstablishedIdleTimeoutSec</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>TcpEstablishedIdleTimeoutSec is the timeout (in seconds) for established TCP connections. Defaults to 1200.</p>
</td>
</tr>
<tr>
<td>
<code>tcpTimeWaitTimeoutSec</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>TcpTimeWaitTimeoutSec is the timeout (in seconds) for TCP connections in &lsquo;TIME_WAIT&rsquo; state. Defaults to 120.</p>
</td>
</tr>
<tr>
<td>
<code>tcpTransitoryIdleTimeoutSec</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>TcpTransitoryIdleTimeoutSec is the timeout (in seconds) for transitory TCP connections. Defaults to 30.</p>
</td>
</tr>
<tr>
<td>
<code>udpIdleTimeoutSec</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>UdpIdleTimeoutSec is the timeout (in seconds) for UDP connections. Defaults to 30.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.CloudRouter">CloudRouter
</h3>
<p>
(<em>Appears on:</em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.VPC">VPC</a>)
</p>
<p>
<p>CloudRouter contains information about the CloudRouter configuration</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the CloudRouter name.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.DataVolume">DataVolume
</h3>
<p>
(<em>Appears on:</em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.WorkerConfig">WorkerConfig</a>)
</p>
<p>
<p>DataVolume contains configuration for data volumes attached to VMs.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the data volume this configuration applies to.</p>
</td>
</tr>
<tr>
<td>
<code>sourceImage</code></br>
<em>
string
</em>
</td>
<td>
<p>SourceImage is the image to create this disk
However, this parameter should only be used with particular caution.
For example GardenLinux works with filesystem LABELs only and creating
another disk form the very same image causes the LABELs to be duplicated.
See: <a href="https://github.com/gardener/gardener-extension-provider-gcp/issues/323">https://github.com/gardener/gardener-extension-provider-gcp/issues/323</a></p>
</td>
</tr>
<tr>
<td>
<code>provisionedIops</code></br>
<em>
int64
</em>
</td>
<td>
<p>ProvisionedIops of disk to create.
Only for certain types of disk, see worker.AllowedTypesIops
The IOPS must be specified within defined limits.
If not set gcp calculates a default value taking the disk size into consideration.
Hyperdisk Extreme volumes can&rsquo;t be used as boot disks.</p>
</td>
</tr>
<tr>
<td>
<code>provisionedThroughput</code></br>
<em>
int64
</em>
</td>
<td>
<p>ProvisionedThroughput of disk to create.
Only for certain types of disk, see worker.AllowedTypesThroughput
measured in MiB per second, that the disk can handle.
The throughput must be specified within defined limits.
If not set gcp calculates a default value taking the disk size into consideration.
Hyperdisk Throughput volumes can&rsquo;t be used as boot disks.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.DiskEncryption">DiskEncryption
</h3>
<p>
(<em>Appears on:</em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.Volume">Volume</a>)
</p>
<p>
<p>DiskEncryption encapsulates the encryption configuration for a disk.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>kmsKeyName</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>KmsKeyName specifies the customer-managed encryption key (CMEK) used for encryption of the volume.
For creating keys, see <a href="https://cloud.google.com/kms/docs/create-key">https://cloud.google.com/kms/docs/create-key</a>.
For using keys to encrypt resources, see:
<a href="https://cloud.google.com/compute/docs/disks/customer-managed-encryption#encrypt_a_new_persistent_disk_with_your_own_keys">https://cloud.google.com/compute/docs/disks/customer-managed-encryption#encrypt_a_new_persistent_disk_with_your_own_keys</a>
This field is being kept optional since this would allow CSEK fields in future in lieu of CMEK fields</p>
</td>
</tr>
<tr>
<td>
<code>kmsKeyServiceAccount</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>KmsKeyServiceAccount specifies the service account granted the <code>roles/cloudkms.cryptoKeyEncrypterDecrypter</code> for the key name.
If nil/empty, then the role should be given to the Compute Engine Service Agent Account. The CESA usually has the format
service-PROJECT_NUMBER@compute-system.iam.gserviceaccount.com.
See: <a href="https://cloud.google.com/iam/docs/service-agents#compute-engine-service-agent">https://cloud.google.com/iam/docs/service-agents#compute-engine-service-agent</a>
One can add IAM roles using the gcloud CLI:
gcloud projects add-iam-policy-binding projectId &ndash;member
serviceAccount:name@projectIdgserviceaccount.com &ndash;role roles/cloudkms.cryptoKeyEncrypterDecrypter</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.EndpointIndependentMapping">EndpointIndependentMapping
</h3>
<p>
(<em>Appears on:</em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.CloudNAT">CloudNAT</a>)
</p>
<p>
<p>EndpointIndependentMapping contains endpoint independent mapping options.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>enabled</code></br>
<em>
bool
</em>
</td>
<td>
<p>Enabled controls if endpoint independent mapping is enabled. Default is false.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.FlowLogs">FlowLogs
</h3>
<p>
(<em>Appears on:</em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.NetworkConfig">NetworkConfig</a>)
</p>
<p>
<p>FlowLogs contains the configuration options for the vpc flow logs.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>aggregationInterval</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>AggregationInterval for collecting flow logs.</p>
</td>
</tr>
<tr>
<td>
<code>flowSampling</code></br>
<em>
float32
</em>
</td>
<td>
<em>(Optional)</em>
<p>FlowSampling sets the sampling rate of VPC flow logs within the subnetwork where 1.0 means all collected logs are reported and 0.0 means no logs are reported.</p>
</td>
</tr>
<tr>
<td>
<code>metadata</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Metadata configures whether metadata fields should be added to the reported VPC flow logs.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.GPU">GPU
</h3>
<p>
(<em>Appears on:</em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.WorkerConfig">WorkerConfig</a>)
</p>
<p>
<p>GPU is the configuration of the GPU to be attached</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>acceleratorType</code></br>
<em>
string
</em>
</td>
<td>
<p>AcceleratorType is the type of accelerator to be attached</p>
</td>
</tr>
<tr>
<td>
<code>count</code></br>
<em>
int32
</em>
</td>
<td>
<p>Count is the number of accelerator to be attached</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.ImmutableConfig">ImmutableConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.BackupBucketConfig">BackupBucketConfig</a>)
</p>
<p>
<p>ImmutableConfig represents the immutability configuration for a backup bucket.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>retentionType</code></br>
<em>
string
</em>
</td>
<td>
<p>RetentionType specifies the type of retention for the backup bucket.
Currently allowed values are:
- &ldquo;bucket&rdquo;: The retention policy applies to the entire bucket.</p>
</td>
</tr>
<tr>
<td>
<code>retentionPeriod</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#duration-v1-meta">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<p>RetentionPeriod specifies the immutability retention period for the backup bucket.
The minimum retention period is 24 hours as per Google Cloud Storage requirements.
Reference: <a href="https://github.com/googleapis/google-cloud-go/blob/3005f5a86c18254e569b8b1782bf014aa62f33cc/storage/bucket.go#L1430-L1434">https://github.com/googleapis/google-cloud-go/blob/3005f5a86c18254e569b8b1782bf014aa62f33cc/storage/bucket.go#L1430-L1434</a></p>
</td>
</tr>
<tr>
<td>
<code>locked</code></br>
<em>
bool
</em>
</td>
<td>
<p>Locked indicates whether the immutable retention policy is locked for the backup bucket.
If set to true, the retention policy cannot be removed or the retention period reduced, enforcing immutability.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.InfrastructureState">InfrastructureState
</h3>
<p>
<p>InfrastructureState contains state information of the infrastructure resource.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>data</code></br>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Data is map to store things.</p>
</td>
</tr>
<tr>
<td>
<code>routes</code></br>
<em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.Route">
[]Route
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Routes contains information about cluster routes</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.InfrastructureStatus">InfrastructureStatus
</h3>
<p>
<p>InfrastructureStatus contains information about created infrastructure resources.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>networks</code></br>
<em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.NetworkStatus">
NetworkStatus
</a>
</em>
</td>
<td>
<p>Networks is the status of the networks of the infrastructure.</p>
</td>
</tr>
<tr>
<td>
<code>serviceAccountEmail</code></br>
<em>
string
</em>
</td>
<td>
<p>ServiceAccountEmail is the email address of the service account.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.MachineImage">MachineImage
</h3>
<p>
(<em>Appears on:</em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.WorkerStatus">WorkerStatus</a>)
</p>
<p>
<p>MachineImage is a mapping from logical names and versions to GCP-specific identifiers.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the logical name of the machine image.</p>
</td>
</tr>
<tr>
<td>
<code>version</code></br>
<em>
string
</em>
</td>
<td>
<p>Version is the logical version of the machine image.</p>
</td>
</tr>
<tr>
<td>
<code>image</code></br>
<em>
string
</em>
</td>
<td>
<p>Image is the path to the image.</p>
</td>
</tr>
<tr>
<td>
<code>architecture</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Architecture is the CPU architecture of the machine image.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.MachineImageVersion">MachineImageVersion
</h3>
<p>
(<em>Appears on:</em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.MachineImages">MachineImages</a>)
</p>
<p>
<p>MachineImageVersion contains a version and a provider-specific identifier.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>version</code></br>
<em>
string
</em>
</td>
<td>
<p>Version is the version of the image.</p>
</td>
</tr>
<tr>
<td>
<code>image</code></br>
<em>
string
</em>
</td>
<td>
<p>Image is the path to the image.</p>
</td>
</tr>
<tr>
<td>
<code>architecture</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Architecture is the CPU architecture of the machine image.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.MachineImages">MachineImages
</h3>
<p>
(<em>Appears on:</em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.CloudProfileConfig">CloudProfileConfig</a>)
</p>
<p>
<p>MachineImages is a mapping from logical names and versions to provider-specific identifiers.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the logical name of the machine image.</p>
</td>
</tr>
<tr>
<td>
<code>versions</code></br>
<em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.MachineImageVersion">
[]MachineImageVersion
</a>
</em>
</td>
<td>
<p>Versions contains versions and a provider-specific identifier.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.NatIP">NatIP
</h3>
<p>
(<em>Appears on:</em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.NetworkStatus">NetworkStatus</a>)
</p>
<p>
<p>NatIP is a user provided external ip which can be used by the nat gateway</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>ip</code></br>
<em>
string
</em>
</td>
<td>
<p>IP is the external premium IP address used in GCP</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.NatIPName">NatIPName
</h3>
<p>
(<em>Appears on:</em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.CloudNAT">CloudNAT</a>)
</p>
<p>
<p>NatIPName is the name of a user provided external ip address which can be used by the nat gateway</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name of the external premium ip address which is used in gcp</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.NetworkConfig">NetworkConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.InfrastructureConfig">InfrastructureConfig</a>)
</p>
<p>
<p>NetworkConfig holds information about the Kubernetes and infrastructure networks.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>vpc</code></br>
<em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.VPC">
VPC
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>VPC indicates whether to use an existing VPC or create a new one.</p>
</td>
</tr>
<tr>
<td>
<code>cloudNAT</code></br>
<em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.CloudNAT">
CloudNAT
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CloudNAT contains configuration about the CloudNAT resource</p>
</td>
</tr>
<tr>
<td>
<code>internal</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Internal is a private subnet (used for internal load balancers).</p>
</td>
</tr>
<tr>
<td>
<code>worker</code></br>
<em>
string
</em>
</td>
<td>
<p>Worker is the worker subnet range to create (used for the VMs).
Deprecated - use <code>workers</code> instead.</p>
</td>
</tr>
<tr>
<td>
<code>workers</code></br>
<em>
string
</em>
</td>
<td>
<p>Workers is the worker subnet range to create (used for the VMs).</p>
</td>
</tr>
<tr>
<td>
<code>flowLogs</code></br>
<em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.FlowLogs">
FlowLogs
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>FlowLogs contains the flow log configuration for the subnet.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.NetworkStatus">NetworkStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.InfrastructureStatus">InfrastructureStatus</a>)
</p>
<p>
<p>NetworkStatus is the current status of the infrastructure networks.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>vpc</code></br>
<em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.VPC">
VPC
</a>
</em>
</td>
<td>
<p>VPC states the name of the infrastructure VPC.</p>
</td>
</tr>
<tr>
<td>
<code>subnets</code></br>
<em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.Subnet">
[]Subnet
</a>
</em>
</td>
<td>
<p>Subnets are the subnets that have been created.</p>
</td>
</tr>
<tr>
<td>
<code>natIPs</code></br>
<em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.NatIP">
[]NatIP
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NatIPs is a list of all user provided external premium ips which can be used by the nat gateway</p>
</td>
</tr>
<tr>
<td>
<code>ipfamilies</code></br>
<em>
[]github.com/gardener/gardener/pkg/apis/core/v1beta1.IPFamily
</em>
</td>
<td>
<p>IPFamilies is the list of the used ip families.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.Route">Route
</h3>
<p>
(<em>Appears on:</em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.InfrastructureState">InfrastructureState</a>)
</p>
<p>
<p>Route is a structure containing information about the routes.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>instanceName</code></br>
<em>
string
</em>
</td>
<td>
<p>InstanceName</p>
</td>
</tr>
<tr>
<td>
<code>destinationCIDR</code></br>
<em>
string
</em>
</td>
<td>
<p>DestinationCIDR</p>
</td>
</tr>
<tr>
<td>
<code>zone</code></br>
<em>
string
</em>
</td>
<td>
<p>Zone is the zone of the route</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.ServiceAccount">ServiceAccount
</h3>
<p>
(<em>Appears on:</em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.WorkerConfig">WorkerConfig</a>)
</p>
<p>
<p>ServiceAccount is a GCP service account.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>email</code></br>
<em>
string
</em>
</td>
<td>
<p>Email is the address of the service account.</p>
</td>
</tr>
<tr>
<td>
<code>scopes</code></br>
<em>
[]string
</em>
</td>
<td>
<p>Scopes is the list of scopes to be made available for this service.
account.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.Storage">Storage
</h3>
<p>
(<em>Appears on:</em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.ControlPlaneConfig">ControlPlaneConfig</a>)
</p>
<p>
<p>Storage contains settings for the default StorageClass and VolumeSnapshotClass</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>managedDefaultStorageClass</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>ManagedDefaultStorageClass controls if the &lsquo;default&rsquo; StorageClass would be marked as default. Set to false to
suppress marking the &lsquo;default&rsquo; StorageClass as default, allowing another StorageClass not managed by Gardener
to be set as default by the user.
Defaults to true.</p>
</td>
</tr>
<tr>
<td>
<code>managedDefaultVolumeSnapshotClass</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>ManagedDefaultVolumeSnapshotClass controls if the &lsquo;default&rsquo; VolumeSnapshotClass would be marked as default.
Set to false to suppress marking the &lsquo;default&rsquo; VolumeSnapshotClass as default, allowing another VolumeSnapshotClass
not managed by Gardener to be set as default by the user.
Defaults to true.</p>
</td>
</tr>
<tr>
<td>
<code>csiFilestore</code></br>
<em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.CSIFilestore">
CSIFilestore
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CSIFilestore contains configuration for CSI Filestore driver (support for NFS volumes)</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.Subnet">Subnet
</h3>
<p>
(<em>Appears on:</em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.NetworkStatus">NetworkStatus</a>)
</p>
<p>
<p>Subnet is a subnet that was created.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the subnet.</p>
</td>
</tr>
<tr>
<td>
<code>purpose</code></br>
<em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.SubnetPurpose">
SubnetPurpose
</a>
</em>
</td>
<td>
<p>Purpose is the purpose for which the subnet was created.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.SubnetPurpose">SubnetPurpose
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.Subnet">Subnet</a>)
</p>
<p>
<p>SubnetPurpose is a purpose of a subnet.</p>
</p>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.VPC">VPC
</h3>
<p>
(<em>Appears on:</em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.NetworkConfig">NetworkConfig</a>, 
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.NetworkStatus">NetworkStatus</a>)
</p>
<p>
<p>VPC contains information about the VPC and some related resources.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the VPC name.</p>
</td>
</tr>
<tr>
<td>
<code>cloudRouter</code></br>
<em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.CloudRouter">
CloudRouter
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CloudRouter indicates whether to use an existing CloudRouter or create a new one</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.Volume">Volume
</h3>
<p>
(<em>Appears on:</em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.WorkerConfig">WorkerConfig</a>)
</p>
<p>
<p>Volume contains configuration for the disks attached to VMs.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>interface</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>LocalSSDInterface is the interface of that the local ssd disk supports.</p>
</td>
</tr>
<tr>
<td>
<code>encryption</code></br>
<em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.DiskEncryption">
DiskEncryption
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Encryption refers to the disk encryption details for this volume</p>
</td>
</tr>
</tbody>
</table>
<h3 id="gcp.provider.extensions.gardener.cloud/v1alpha1.WorkerStatus">WorkerStatus
</h3>
<p>
<p>WorkerStatus contains information about created worker resources.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>machineImages</code></br>
<em>
<a href="#gcp.provider.extensions.gardener.cloud/v1alpha1.MachineImage">
[]MachineImage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineImages is a list of machine images that have been used in this worker. Usually, the extension controller
gets the mapping from name/version to the provider-specific machine image data in its componentconfig. However, if
a version that is still in use gets removed from this componentconfig it cannot reconcile anymore existing <code>Worker</code>
resources that are still using this version. Hence, it stores the used versions in the provider status to ensure
reconciliation is possible.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <a href="https://github.com/ahmetb/gen-crd-api-reference-docs">gen-crd-api-reference-docs</a>
</em></p>
