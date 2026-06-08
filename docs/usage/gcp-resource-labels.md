# GCP Resource Labels

This document describes the labels applied by `gardener-extension-provider-gcp` to GCP resources, and the label sanitization rules that apply to worker pool labels propagated to VM instances.

## Worker Nodes (Compute Engine VM Instances)

Worker node VMs receive labels from two sources: fixed labels set by the extension and labels derived from the worker pool configuration.

### Fixed Labels

| Label Key | Label Value | Description |
|---|---|---|
| `name` | Sanitized shoot name | Identifies the shoot the node belongs to |
| `k8s-cluster-name` | Sanitized Technical ID of the Shoot | Ensures consistency with the label added to all disks by the CSI driver |

### Worker Pool Labels

All labels configured in the worker pool's `.spec.pools[].labels` field are propagated to the corresponding VM instances after sanitization (see [Label Sanitization](#label-sanitization) below).

### VM Network Tags

In addition to labels, GCP VM network tags are set on worker node instances. Network tags are used by GCP firewall rules to control traffic.

| Tag | Description |
|---|---|
| `<shoot-technical-id>` | Technical ID of the Shoot, used to identify the cluster |
| `kubernetes-io-cluster-<shoot-technical-id>` | Kubernetes cluster identifier tag |
| `kubernetes-io-role-node` | Marks the instance as a Kubernetes worker node |

### Disks

Boot volumes and data volumes attached to worker nodes receive the same labels as the VM instance they are attached to (i.e., the fixed labels and sanitized worker pool labels described above).

## Bastion Instances

Bastion host VMs are created with a single network tag set to the bastion instance name. This tag is used to scope the dedicated firewall ingress and egress rules to the bastion instance only.

| Tag | Value |
|---|---|
| Network tag | `<bastion-instance-name>` (derived from cluster name and bastion name) |

## Label Sanitization

GCP labels have restrictions on allowed characters. Both label keys and values must:

- Contain only lowercase letters (`a-z`), digits (`0-9`), hyphens (`-`), and underscores (`_`)
- Be at most 63 characters long

The extension sanitizes worker pool label keys and values before applying them to VM instances and disks:

1. The key/value is converted to lowercase.
2. Any character not matching `[a-z0-9_-]` is replaced with an underscore (`_`).
3. For label keys only: leading digits and underscores are stripped, since GCP requires keys to start with a letter.
4. Keys or values that exceed 63 characters are truncated to 63 characters.
5. Keys that are empty after sanitization are dropped entirely.

### Example

Given a worker pool with the following labels in the Shoot spec:

```yaml
spec:
  provider:
    workers:
    - name: worker-pool-1
      labels:
        worker.gardener.cloud/pool: worker-pool-1
        node.gardener.cloud/critical-components-only: "true"
        example.com/my-label: my-value
```

The labels applied to the corresponding VM instances would be:

```text
worker_gardener_cloud_pool = worker-pool-1
node_gardener_cloud_critical-components-only = true
example_com_my-label = my-value
```
