<p>Packages:</p>
<ul>
<li>
<a href="#gcp.provider.extensions.gardener.cloud%2fv1alpha1">gcp.provider.extensions.gardener.cloud/v1alpha1</a>
</li>
</ul>

<h2 id="gcp.provider.extensions.gardener.cloud/v1alpha1">gcp.provider.extensions.gardener.cloud/v1alpha1</h2>
<p>

</p>
Resource Types:
<ul>
<li>
<a href="#flowlogs">FlowLogs</a>
</li>
</ul>

<h3 id="backupbucketconfig">BackupBucketConfig
</h3>


<p>
BackupBucketConfig represents the configuration for a backup bucket.
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
<code>immutability</code></br>
<em>
<a href="#immutableconfig">ImmutableConfig</a>
</em>
</td>
<td>
<p>Immutability defines the immutability config for the backup bucket.</p>
</td>
</tr>
<tr>
<td>
<code>store</code></br>
<em>
<a href="#store">Store</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Store holds the configuration of the backup store</p>
</td>
</tr>

</tbody>
</table>


<h3 id="bootvolume">BootVolume
</h3>


<p>
(<em>Appears on:</em><a href="#workerconfig">WorkerConfig</a>)
</p>

<p>
BootVolume contains configuration for the boot volume attached to VMs.
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
<code>DiskSettings</code></br>
<em>
<a href="#disksettings">DiskSettings</a>
</em>
</td>
<td>
<p></p>
</td>
</tr>

</tbody>
</table>


<h3 id="csifilestore">CSIFilestore
</h3>


<p>
(<em>Appears on:</em><a href="#storage">Storage</a>)
</p>

<p>
CSIFilestore contains configuration for CSI Filestore driver
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
boolean
</em>
</td>
<td>
<p>Enabled is the switch to enable the CSI Manila driver support</p>
</td>
</tr>

</tbody>
</table>


<h3 id="cloudcontrollermanagerconfig">CloudControllerManagerConfig
</h3>


<p>
(<em>Appears on:</em><a href="#controlplaneconfig">ControlPlaneConfig</a>)
</p>

<p>
CloudControllerManagerConfig contains configuration settings for the cloud-controller-manager.
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
object (keys:string, values:boolean)
</em>
</td>
<td>
<em>(Optional)</em>
<p>FeatureGates contains information about enabled feature gates.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="cloudnat">CloudNAT
</h3>


<p>
(<em>Appears on:</em><a href="#networkconfig">NetworkConfig</a>)
</p>

<p>
CloudNAT contains configuration about the CloudNAT resource
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
<a href="#endpointindependentmapping">EndpointIndependentMapping</a>
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
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>MinPortsPerVM is the minimum number of ports allocated to a VM in the NAT config.<br />The default value is 2048 ports.</p>
</td>
</tr>
<tr>
<td>
<code>maxPortsPerVM</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxPortsPerVM is the maximum number of ports allocated to a VM in the NAT config.<br />The default value is 65536 ports.</p>
</td>
</tr>
<tr>
<td>
<code>enableDynamicPortAllocation</code></br>
<em>
boolean
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
<a href="#natipname">NatIPName</a> array
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
integer
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
integer
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
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>TcpTimeWaitTimeoutSec is the timeout (in seconds) for TCP connections in 'TIME_WAIT' state. Defaults to 120.</p>
</td>
</tr>
<tr>
<td>
<code>tcpTransitoryIdleTimeoutSec</code></br>
<em>
integer
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
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>UdpIdleTimeoutSec is the timeout (in seconds) for UDP connections. Defaults to 30.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="cloudprofileconfig">CloudProfileConfig
</h3>


<p>
CloudProfileConfig contains provider-specific configuration that is embedded into Gardener's `CloudProfile`
resource.
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
<a href="#machineimages">MachineImages</a> array
</em>
</td>
<td>
<p>MachineImages is the list of machine images that are understood by the controller. It maps<br />logical names and versions to provider-specific identifiers.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="cloudrouter">CloudRouter
</h3>


<p>
(<em>Appears on:</em><a href="#vpc">VPC</a>)
</p>

<p>
CloudRouter contains information about the CloudRouter configuration
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


<h3 id="controlplaneconfig">ControlPlaneConfig
</h3>


<p>
ControlPlaneConfig contains configuration settings for the control plane.
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
<a href="#cloudcontrollermanagerconfig">CloudControllerManagerConfig</a>
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
<a href="#storage">Storage</a>
</em>
</td>
<td>
<p>Storage contains configuration for the storage in the cluster.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="datavolume">DataVolume
</h3>


<p>
(<em>Appears on:</em><a href="#workerconfig">WorkerConfig</a>)
</p>

<p>
DataVolume contains configuration for data volumes attached to VMs.
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
<em>(Optional)</em>
<p>SourceImage is the image to create this disk<br />However, this parameter should only be used with particular caution.<br />For example GardenLinux works with filesystem LABELs only and creating<br />another disk form the very same image causes the LABELs to be duplicated.<br />See: https://github.com/gardener/gardener-extension-provider-gcp/issues/323</p>
</td>
</tr>
<tr>
<td>
<code>DiskSettings</code></br>
<em>
<a href="#disksettings">DiskSettings</a>
</em>
</td>
<td>
<p></p>
</td>
</tr>

</tbody>
</table>


<h3 id="diskencryption">DiskEncryption
</h3>


<p>
(<em>Appears on:</em><a href="#volume">Volume</a>)
</p>

<p>
DiskEncryption encapsulates the encryption configuration for a disk.
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
<p>KmsKeyName specifies the customer-managed encryption key (CMEK) used for encryption of the volume.<br />For creating keys, see https://cloud.google.com/kms/docs/create-key.<br />For using keys to encrypt resources, see:<br />https://cloud.google.com/compute/docs/disks/customer-managed-encryption#encrypt_a_new_persistent_disk_with_your_own_keys<br />This field is being kept optional since this would allow CSEK fields in future in lieu of CMEK fields</p>
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
<p>KmsKeyServiceAccount specifies the service account granted the `roles/cloudkms.cryptoKeyEncrypterDecrypter` for the key name.<br />If nil/empty, then the role should be given to the Compute Engine Service Agent Account. The CESA usually has the format<br />service-PROJECT_NUMBER@compute-system.iam.gserviceaccount.com.<br /> See: https://cloud.google.com/iam/docs/service-agents#compute-engine-service-agent<br />One can add IAM roles using the gcloud CLI:<br /> gcloud projects add-iam-policy-binding projectId --member<br />	serviceAccount:name@projectIdgserviceaccount.com --role roles/cloudkms.cryptoKeyEncrypterDecrypter</p>
</td>
</tr>

</tbody>
</table>


<h3 id="disksettings">DiskSettings
</h3>


<p>
(<em>Appears on:</em><a href="#bootvolume">BootVolume</a>, <a href="#datavolume">DataVolume</a>)
</p>

<p>
DiskSettings stores single disk specific information.
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
<code>provisionedIops</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProvisionedIops of disk to create.<br />Only for certain types of disk, see worker.AllowedTypesIops<br />The IOPS must be specified within defined limits.<br />If not set gcp calculates a default value taking the disk size into consideration.<br />Hyperdisk Extreme volumes can't be used as boot disks.</p>
</td>
</tr>
<tr>
<td>
<code>provisionedThroughput</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProvisionedThroughput of disk to create.<br />Only for certain types of disk, see worker.AllowedTypesThroughput<br />measured in MiB per second, that the disk can handle.<br />The throughput must be specified within defined limits.<br />If not set gcp calculates a default value taking the disk size into consideration.<br />Hyperdisk Throughput volumes can't be used as boot disks.</p>
</td>
</tr>
<tr>
<td>
<code>storagePool</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>StoragePool in which the new disk is created.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="endpointindependentmapping">EndpointIndependentMapping
</h3>


<p>
(<em>Appears on:</em><a href="#cloudnat">CloudNAT</a>)
</p>

<p>
EndpointIndependentMapping contains endpoint independent mapping options.
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
boolean
</em>
</td>
<td>
<p>Enabled controls if endpoint independent mapping is enabled. Default is false.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="flowlogs">FlowLogs
</h3>


<p>
(<em>Appears on:</em><a href="#networkconfig">NetworkConfig</a>)
</p>

<p>
FlowLogs contains the configuration options for the vpc flow logs.
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
float
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
Refer to the Kubernetes API documentation for the fields of the <code>metadata</code> field.
</td>
</tr>

</tbody>
</table>


<h3 id="gpu">GPU
</h3>


<p>
(<em>Appears on:</em><a href="#workerconfig">WorkerConfig</a>)
</p>

<p>
GPU is the configuration of the GPU to be attached
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
integer
</em>
</td>
<td>
<p>Count is the number of accelerator to be attached</p>
</td>
</tr>

</tbody>
</table>


<h3 id="immutableconfig">ImmutableConfig
</h3>


<p>
(<em>Appears on:</em><a href="#backupbucketconfig">BackupBucketConfig</a>)
</p>

<p>
ImmutableConfig represents the immutability configuration for a backup bucket.
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
<p>RetentionType specifies the type of retention for the backup bucket.<br />Currently allowed values are:<br />- "bucket": The retention policy applies to the entire bucket.</p>
</td>
</tr>
<tr>
<td>
<code>retentionPeriod</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<p>RetentionPeriod specifies the immutability retention period for the backup bucket.<br />The minimum retention period is 24 hours as per Google Cloud Storage requirements.<br />Reference: https://github.com/googleapis/google-cloud-go/blob/3005f5a86c18254e569b8b1782bf014aa62f33cc/storage/bucket.go#L1430-L1434</p>
</td>
</tr>
<tr>
<td>
<code>locked</code></br>
<em>
boolean
</em>
</td>
<td>
<p>Locked indicates whether the immutable retention policy is locked for the backup bucket.<br />If set to true, the retention policy cannot be removed or the retention period reduced, enforcing immutability.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="infrastructureconfig">InfrastructureConfig
</h3>


<p>
InfrastructureConfig infrastructure configuration resource
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
<a href="#networkconfig">NetworkConfig</a>
</em>
</td>
<td>
<p>Networks is the network configuration (VPC, subnets, etc.)</p>
</td>
</tr>

</tbody>
</table>


<h3 id="infrastructurestate">InfrastructureState
</h3>


<p>
InfrastructureState contains state information of the infrastructure resource.
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
object (keys:string, values:string)
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
<a href="#route">Route</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Routes contains information about cluster routes</p>
</td>
</tr>

</tbody>
</table>


<h3 id="infrastructurestatus">InfrastructureStatus
</h3>


<p>
InfrastructureStatus contains information about created infrastructure resources.
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
<a href="#networkstatus">NetworkStatus</a>
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


<h3 id="machineimage">MachineImage
</h3>


<p>
(<em>Appears on:</em><a href="#workerstatus">WorkerStatus</a>)
</p>

<p>
MachineImage is a mapping from logical names and versions to GCP-specific identifiers.
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
<tr>
<td>
<code>capabilities</code></br>
<em>
<a href="#capabilities">Capabilities</a>
</em>
</td>
<td>
<p>Capabilities of the machine image.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machineimageflavor">MachineImageFlavor
</h3>


<p>
(<em>Appears on:</em><a href="#machineimageversion">MachineImageVersion</a>)
</p>

<p>
MachineImageFlavor is a flavor of the machine image version that supports a specific set of capabilities.
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
<code>capabilities</code></br>
<em>
<a href="#capabilities">Capabilities</a>
</em>
</td>
<td>
<p>Capabilities is the set of capabilities that are supported by the AMIs in this set.</p>
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

</tbody>
</table>


<h3 id="machineimageversion">MachineImageVersion
</h3>


<p>
(<em>Appears on:</em><a href="#machineimages">MachineImages</a>)
</p>

<p>
MachineImageVersion contains a version and a provider-specific identifier.
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
<em>(Optional)</em>
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
<tr>
<td>
<code>capabilityFlavors</code></br>
<em>
<a href="#machineimageflavor">MachineImageFlavor</a> array
</em>
</td>
<td>
<p>CapabilityFlavors is a collection of all images for that version with capabilities.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machineimages">MachineImages
</h3>


<p>
(<em>Appears on:</em><a href="#cloudprofileconfig">CloudProfileConfig</a>)
</p>

<p>
MachineImages is a mapping from logical names and versions to provider-specific identifiers.
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
<a href="#machineimageversion">MachineImageVersion</a> array
</em>
</td>
<td>
<p>Versions contains versions and a provider-specific identifier.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="natip">NatIP
</h3>


<p>
(<em>Appears on:</em><a href="#networkstatus">NetworkStatus</a>)
</p>

<p>
NatIP is a user provided external ip which can be used by the nat gateway
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


<h3 id="natipname">NatIPName
</h3>


<p>
(<em>Appears on:</em><a href="#cloudnat">CloudNAT</a>)
</p>

<p>
NatIPName is the name of a user provided external ip address which can be used by the nat gateway
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


<h3 id="networkconfig">NetworkConfig
</h3>


<p>
(<em>Appears on:</em><a href="#infrastructureconfig">InfrastructureConfig</a>)
</p>

<p>
NetworkConfig holds information about the Kubernetes and infrastructure networks.
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
<a href="#vpc">VPC</a>
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
<a href="#cloudnat">CloudNAT</a>
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
<p>Worker is the worker subnet range to create (used for the VMs).<br />Deprecated - use `workers` instead.</p>
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
<a href="#flowlogs">FlowLogs</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>FlowLogs contains the flow log configuration for the subnet.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="networkstatus">NetworkStatus
</h3>


<p>
(<em>Appears on:</em><a href="#infrastructurestatus">InfrastructureStatus</a>)
</p>

<p>
NetworkStatus is the current status of the infrastructure networks.
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
<a href="#vpc">VPC</a>
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
<a href="#subnet">Subnet</a> array
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
<a href="#natip">NatIP</a> array
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
IPFamily array
</em>
</td>
<td>
<p>IPFamilies is the list of the used ip families.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="route">Route
</h3>


<p>
(<em>Appears on:</em><a href="#infrastructurestate">InfrastructureState</a>)
</p>

<p>
Route is a structure containing information about the routes.
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


<h3 id="serviceaccount">ServiceAccount
</h3>


<p>
(<em>Appears on:</em><a href="#workerconfig">WorkerConfig</a>)
</p>

<p>
ServiceAccount is a GCP service account.
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
string array
</em>
</td>
<td>
<p>Scopes is the list of scopes to be made available for this service.<br />account.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="storage">Storage
</h3>


<p>
(<em>Appears on:</em><a href="#controlplaneconfig">ControlPlaneConfig</a>)
</p>

<p>
Storage contains settings for the default StorageClass and VolumeSnapshotClass
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
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>ManagedDefaultStorageClass controls if the 'default' StorageClass would be marked as default. Set to false to<br />suppress marking the 'default' StorageClass as default, allowing another StorageClass not managed by Gardener<br />to be set as default by the user.<br />Defaults to true.</p>
</td>
</tr>
<tr>
<td>
<code>managedDefaultVolumeSnapshotClass</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>ManagedDefaultVolumeSnapshotClass controls if the 'default' VolumeSnapshotClass would be marked as default.<br />Set to false to suppress marking the 'default' VolumeSnapshotClass as default, allowing another VolumeSnapshotClass<br />not managed by Gardener to be set as default by the user.<br />Defaults to true.</p>
</td>
</tr>
<tr>
<td>
<code>csiFilestore</code></br>
<em>
<a href="#csifilestore">CSIFilestore</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CSIFilestore contains configuration for CSI Filestore driver (support for NFS volumes)</p>
</td>
</tr>

</tbody>
</table>


<h3 id="store">Store
</h3>


<p>
(<em>Appears on:</em><a href="#backupbucketconfig">BackupBucketConfig</a>)
</p>

<p>
Store holds the configuration of the backup store
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
<code>endpointOverride</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>EndpointOverride specifies the overriding endpoint at which the GCS bucket is hosted. Necessary for regional endpoints.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="subnet">Subnet
</h3>


<p>
(<em>Appears on:</em><a href="#networkstatus">NetworkStatus</a>)
</p>

<p>
Subnet is a subnet that was created.
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
<a href="#subnetpurpose">SubnetPurpose</a>
</em>
</td>
<td>
<p>Purpose is the purpose for which the subnet was created.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="subnetpurpose">SubnetPurpose
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#subnet">Subnet</a>)
</p>

<p>
SubnetPurpose is a purpose of a subnet.
</p>


<h3 id="vpc">VPC
</h3>


<p>
(<em>Appears on:</em><a href="#networkconfig">NetworkConfig</a>, <a href="#networkstatus">NetworkStatus</a>)
</p>

<p>
VPC contains information about the VPC and some related resources.
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
<a href="#cloudrouter">CloudRouter</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CloudRouter indicates whether to use an existing CloudRouter or create a new one</p>
</td>
</tr>

</tbody>
</table>


<h3 id="volume">Volume
</h3>


<p>
(<em>Appears on:</em><a href="#workerconfig">WorkerConfig</a>)
</p>

<p>
Volume contains general configuration for all disks attached to VMs.
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
<a href="#diskencryption">DiskEncryption</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Encryption refers to the disk encryption details</p>
</td>
</tr>

</tbody>
</table>


<h3 id="workerconfig">WorkerConfig
</h3>


<p>
WorkerConfig contains configuration settings for the worker nodes.
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
<code>gpu</code></br>
<em>
<a href="#gpu">GPU</a>
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
<a href="#volume">Volume</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Volume contains general configuration for the disks attached to VMs.</p>
</td>
</tr>
<tr>
<td>
<code>bootVolume</code></br>
<em>
<a href="#bootvolume">BootVolume</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>BootVolume contains configuration for the root disks attached to VMs.</p>
</td>
</tr>
<tr>
<td>
<code>dataVolumes</code></br>
<em>
<a href="#datavolume">DataVolume</a> array
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
<em>(Optional)</em>
<p>MinCpuPlatform is the name of the minimum CPU platform that is to be<br />requested for the VM.</p>
</td>
</tr>
<tr>
<td>
<code>serviceAccount</code></br>
<em>
<a href="#serviceaccount">ServiceAccount</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Service account, with their specified scopes, authorized for this worker.<br />Service accounts generate access tokens that can be accessed through<br />the metadata server and used to authenticate applications on the<br />instance.<br />This service account should be created in advance.</p>
</td>
</tr>
<tr>
<td>
<code>nodeTemplate</code></br>
<em>
<a href="#nodetemplate">NodeTemplate</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeTemplate contains resource information of the machine which is used by Cluster Autoscaler to generate nodeTemplate during scaling a nodeGroup from zero.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="workerstatus">WorkerStatus
</h3>


<p>
WorkerStatus contains information about created worker resources.
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
<a href="#machineimage">MachineImage</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineImages is a list of machine images that have been used in this worker. Usually, the extension controller<br />gets the mapping from name/version to the provider-specific machine image data in its componentconfig. However, if<br />a version that is still in use gets removed from this componentconfig it cannot reconcile anymore existing `Worker`<br />resources that are still using this version. Hence, it stores the used versions in the provider status to ensure<br />reconciliation is possible.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="workloadidentityconfig">WorkloadIdentityConfig
</h3>


<p>
WorkloadIdentityConfig contains configuration settings for workload identity.
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<p>CredentialsConfig contains information for workload authentication against GCP.</p>
</td>
</tr>

</tbody>
</table>


