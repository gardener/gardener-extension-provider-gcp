<p>Packages:</p>
<ul>
<li>
<a href="#gcp.provider.extensions.config.gardener.cloud%2fv1alpha1">gcp.provider.extensions.config.gardener.cloud/v1alpha1</a>
</li>
</ul>

<h2 id="gcp.provider.extensions.config.gardener.cloud/v1alpha1">gcp.provider.extensions.config.gardener.cloud/v1alpha1</h2>
<p>

</p>

<h3 id="controllerconfiguration">ControllerConfiguration
</h3>


<p>
ControllerConfiguration defines the configuration for the GCP provider.
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
<code>clientConnection</code></br>
<em>
<a href="#clientconnectionconfiguration">ClientConnectionConfiguration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ClientConnection specifies the kubeconfig file and client connection<br />settings for the proxy server to use when communicating with the apiserver.</p>
</td>
</tr>
<tr>
<td>
<code>etcd</code></br>
<em>
<a href="#etcd">ETCD</a>
</em>
</td>
<td>
<p>ETCD is the etcd configuration.</p>
</td>
</tr>
<tr>
<td>
<code>healthCheckConfig</code></br>
<em>
<a href="#healthcheckconfig">HealthCheckConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>HealthCheckConfig is the config for the health check controller</p>
</td>
</tr>
<tr>
<td>
<code>featureGates</code></br>
<em>
object (keys:string, values:boolean)
</em>
</td>
<td>
<em>(Optional)</em>
<p>FeatureGates is a map of feature names to bools that enable<br />or disable alpha/experimental features.<br />Default: nil</p>
</td>
</tr>
<tr>
<td>
<code>profiling</code></br>
<em>
<a href="#profilingconfiguration">ProfilingConfiguration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Profiling holds configuration for profiling and debugging related features.<br />This configuration is meant for debugging purposes only<br />and should be used in production with caution<br />as pprof can expose sensitive information and impact performance.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="etcd">ETCD
</h3>


<p>
(<em>Appears on:</em><a href="#controllerconfiguration">ControllerConfiguration</a>)
</p>

<p>
ETCD is an etcd configuration.
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
<code>storage</code></br>
<em>
<a href="#etcdstorage">ETCDStorage</a>
</em>
</td>
<td>
<p>ETCDStorage is the etcd storage configuration.</p>
</td>
</tr>
<tr>
<td>
<code>backup</code></br>
<em>
<a href="#etcdbackup">ETCDBackup</a>
</em>
</td>
<td>
<p>ETCDBackup is the etcd backup configuration.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="etcdbackup">ETCDBackup
</h3>


<p>
(<em>Appears on:</em><a href="#etcd">ETCD</a>)
</p>

<p>
ETCDBackup is an etcd backup configuration.
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
<code>schedule</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Schedule is the etcd backup schedule.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="etcdstorage">ETCDStorage
</h3>


<p>
(<em>Appears on:</em><a href="#etcd">ETCD</a>)
</p>

<p>
ETCDStorage is an etcd storage configuration.
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
<code>className</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ClassName is the name of the storage class used in etcd-main volume claims.</p>
</td>
</tr>
<tr>
<td>
<code>capacity</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#quantity-resource-api">Quantity</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Capacity is the storage capacity used in etcd-main volume claims.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="profilingconfiguration">ProfilingConfiguration
</h3>


<p>
(<em>Appears on:</em><a href="#controllerconfiguration">ControllerConfiguration</a>)
</p>

<p>
ProfilingConfiguration contains debugging and profiling configuration.
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
<code>pprofBindAddress</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>PprofBindAddress is the TCP address that the controller should bind to for serving pprof.</p>
</td>
</tr>
<tr>
<td>
<code>enableContentionProfiling</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>EnableContentionProfiling enables block profiling, if PprofBindAddress is set.</p>
</td>
</tr>

</tbody>
</table>


