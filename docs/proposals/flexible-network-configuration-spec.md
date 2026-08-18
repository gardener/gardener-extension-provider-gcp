# Implementation Spec: Bring-Your-Own Subnet for User-Managed Egress on GCP

**Status**: implementation checklist derived from [flexible-network-configuration-proposal.md](./flexible-network-configuration-proposal.md). The proposal is the source of truth for design intent and constraints; this spec captures the concrete file changes, code sketches, and test surface needed to satisfy the proposal's acceptance criteria.

**Audience**: coding agents and implementing engineers. Anything in this document may be revised during implementation as long as the changes still satisfy the proposal's acceptance criteria (referenced below by their proposal IDs — `A1`–`A4`, `B1`–`B5`, `C1`–`C17`, `D1`–`D4`, `E1`–`E9`, `F1`–`F3`).

<!-- toc -->

- [Scope and non-scope](#scope-and-non-scope)
- [API type changes](#api-type-changes)
- [Validation](#validation)
- [Reconciler](#reconciler)
- [Cloud-provider config](#cloud-provider-config)
- [Bastion controller](#bastion-controller)
- [Metadata labels](#metadata-labels)
- [Testing plan](#testing-plan)
- [Suggested implementation order](#suggested-implementation-order)

<!-- /toc -->

## Scope and non-scope

**In scope for the initial PR**: everything below. Satisfies acceptance criteria groups A–G in the proposal.

**Out of scope, deferred to follow-up PRs**: explicit `OutboundType` enum surface; in-place transition between managed and BYO mode on an existing shoot; `SkipRouteReconciliation` for overlay-CNI shoots; tightening managed-mode static firewall rules to be tag-scoped. All are enumerated in the proposal's "Out of scope" section and must be actively rejected by validation (see `D1`, `D2`) or simply absent from the reconciler.

**Contract with the design**: if any of the following implementation details turn out to be wrong at coding time (e.g. a file path has shifted, an API signature is different), revise the spec — do not silently deviate. If the deviation touches proposal-level design intent (a new mode, a different API shape, a different firewall contract), stop and get the proposal amended first.

## API type changes

**Files**:
- `pkg/apis/gcp/types_infrastructure.go` (internal types)
- `pkg/apis/gcp/v1alpha1/types_infrastructure.go` (v1alpha1 types with JSON tags)

Add `SubnetReference` type and two new optional fields on `NetworkConfig`:

```go
// SubnetReference is a reference to a user-managed GCP subnetwork.
type SubnetReference struct {
    // Name is the name of the subnetwork.
    Name string `json:"name"`

    // PodSecondaryRangeName is the name of the secondary IP range on the nodes subnet
    // that is used for pod IPs (required for dual-stack BYO shoots).
    // +optional
    PodSecondaryRangeName *string `json:"podSecondaryRangeName,omitempty"`
}

// NetworkConfig — add alongside existing fields:

// SubnetWorkers is a reference to a user-managed subnet for worker nodes.
// When set, the extension operates in BYO (bring-your-own) subnet mode:
// Gardener does not create or delete the subnet; it only attaches to it.
// VPC.Name must be set; CloudNAT, Internal, Worker/Workers, FlowLogs, and MTU must not be set.
// +optional
SubnetWorkers *SubnetReference `json:"subnetWorkers,omitempty"`

// SubnetServices is a reference to a user-managed subnet for services (required for dual-stack BYO shoots).
// Only valid when SubnetWorkers is set.
// +optional
SubnetServices *SubnetReference `json:"subnetServices,omitempty"`
```

Add helper method on `InfrastructureConfig` (internal type only; v1alpha1 uses conversion):

```go
// IsUserManagedEgress reports whether the shoot opts into BYO subnetworks and
// user-managed egress (i.e., SubnetWorkers is set), meaning the extension does
// not manage egress resources.
func (i *InfrastructureConfig) IsUserManagedEgress() bool {
    return i.Networks.SubnetWorkers != nil
}
```

After adding fields, regenerate deep-copy and conversion code:

```bash
make generate
```

## Validation

### API-level validation

**File**: `pkg/apis/gcp/validation/infrastructure.go`

Extend `ValidateInfrastructureConfig` with an `ipFamilies []core.IPFamily` parameter (added after `servicesCIDR`) and a new block that fires when `infra.Networks.SubnetWorkers != nil`. Update the call site in `pkg/admission/validator/shoot.go` to pass `valContext.shoot.Spec.Networking.IPFamilies`:

```go
if infra.Networks.SubnetWorkers != nil {
    byoPath := fldPath.Child("networks")

    // VPC.Name required
    if infra.Networks.VPC == nil || len(infra.Networks.VPC.Name) == 0 {
        allErrs = append(allErrs, field.Required(byoPath.Child("vpc", "name"),
            "must be set when subnetWorkers is provided"))
    }
    // CloudRouter forbidden
    if infra.Networks.VPC != nil && infra.Networks.VPC.CloudRouter != nil {
        allErrs = append(allErrs, field.Forbidden(byoPath.Child("vpc", "cloudRouter"),
            "must not be set when subnetWorkers is provided"))
    }
    // Workers / Worker forbidden
    if len(infra.Networks.Workers) > 0 {
        allErrs = append(allErrs, field.Forbidden(byoPath.Child("workers"),
            "must not be set when subnetWorkers is provided"))
    }
    if len(infra.Networks.Worker) > 0 {
        allErrs = append(allErrs, field.Forbidden(byoPath.Child("worker"),
            "must not be set when subnetWorkers is provided"))
    }
    // Internal forbidden
    if infra.Networks.Internal != nil {
        allErrs = append(allErrs, field.Forbidden(byoPath.Child("internal"),
            "must not be set when subnetWorkers is provided"))
    }
    // CloudNAT forbidden
    if infra.Networks.CloudNAT != nil {
        allErrs = append(allErrs, field.Forbidden(byoPath.Child("cloudNAT"),
            "must not be set when subnetWorkers is provided"))
    }
    // FlowLogs forbidden
    if infra.Networks.FlowLogs != nil {
        allErrs = append(allErrs, field.Forbidden(byoPath.Child("flowLogs"),
            "must not be set when subnetWorkers is provided"))
    }
    // MTU forbidden
    if infra.Networks.MTU != nil {
        allErrs = append(allErrs, field.Forbidden(byoPath.Child("mtu"),
            "must not be set when subnetWorkers is provided"))
    }
    // SubnetWorkers.Name required
    if len(infra.Networks.SubnetWorkers.Name) == 0 {
        allErrs = append(allErrs, field.Required(byoPath.Child("subnetWorkers", "name"),
            "must not be empty"))
    } else {
        allErrs = append(allErrs, validateGcpResourceName(infra.Networks.SubnetWorkers.Name,
            byoPath.Child("subnetWorkers", "name"))...)
    }

    isDualStack := slices.Contains(ipFamilies, core.IPFamilyIPv6)

    // PodSecondaryRangeName: forbidden for IPv4, required and non-empty for dual-stack
    podRangePath := byoPath.Child("subnetWorkers", "podSecondaryRangeName")
    if !isDualStack && infra.Networks.SubnetWorkers.PodSecondaryRangeName != nil {
        allErrs = append(allErrs, field.Forbidden(podRangePath,
            "must not be set for single-stack IPv4 shoots"))
    }
    if isDualStack && infra.Networks.SubnetWorkers.PodSecondaryRangeName == nil {
        allErrs = append(allErrs, field.Required(podRangePath,
            "required for dual-stack shoots"))
    }
    if isDualStack && infra.Networks.SubnetWorkers.PodSecondaryRangeName != nil &&
        len(*infra.Networks.SubnetWorkers.PodSecondaryRangeName) == 0 {
        allErrs = append(allErrs, field.Invalid(podRangePath, "",
            "must not be empty"))
    }

    // SubnetServices: required for dual-stack, forbidden for IPv4
    if isDualStack && infra.Networks.SubnetServices == nil {
        allErrs = append(allErrs, field.Required(byoPath.Child("subnetServices"),
            "required for dual-stack shoots when subnetWorkers is set"))
    }
    if !isDualStack && infra.Networks.SubnetServices != nil {
        allErrs = append(allErrs, field.Forbidden(byoPath.Child("subnetServices"),
            "must not be set for single-stack IPv4 shoots"))
    }
    if isDualStack && infra.Networks.SubnetServices != nil {
        if len(infra.Networks.SubnetServices.Name) == 0 {
            allErrs = append(allErrs, field.Required(byoPath.Child("subnetServices", "name"),
                "must not be empty"))
        }
        if infra.Networks.SubnetServices.PodSecondaryRangeName != nil {
            allErrs = append(allErrs, field.Forbidden(byoPath.Child("subnetServices", "podSecondaryRangeName"),
                "only valid on subnetWorkers, not subnetServices"))
        }
    }
}

// SubnetServices without SubnetWorkers
if infra.Networks.SubnetServices != nil && infra.Networks.SubnetWorkers == nil {
    allErrs = append(allErrs, field.Forbidden(fldPath.Child("networks", "subnetServices"),
        "must not be set without subnetWorkers"))
}
```

Extend `ValidateInfrastructureConfigUpdate` in the same file with immutability rules:

```go
// Mode transition forbidden
oldBYO := oldConfig.Networks.SubnetWorkers != nil
newBYO := newConfig.Networks.SubnetWorkers != nil
if oldBYO != newBYO {
    allErrs = append(allErrs, field.Forbidden(fldPath.Child("networks", "subnetWorkers"),
        "cannot add or remove subnetWorkers on an existing shoot"))
}
// SubnetWorkers.Name immutable
if oldConfig.Networks.SubnetWorkers != nil && newConfig.Networks.SubnetWorkers != nil {
    allErrs = append(allErrs, apivalidation.ValidateImmutableField(
        newConfig.Networks.SubnetWorkers.Name,
        oldConfig.Networks.SubnetWorkers.Name,
        fldPath.Child("networks", "subnetWorkers", "name"))...)
    allErrs = append(allErrs, apivalidation.ValidateImmutableField(
        newConfig.Networks.SubnetWorkers.PodSecondaryRangeName,
        oldConfig.Networks.SubnetWorkers.PodSecondaryRangeName,
        fldPath.Child("networks", "subnetWorkers", "podSecondaryRangeName"))...)
}
// SubnetServices.Name immutable
if oldConfig.Networks.SubnetServices != nil && newConfig.Networks.SubnetServices != nil {
    allErrs = append(allErrs, apivalidation.ValidateImmutableField(
        newConfig.Networks.SubnetServices.Name,
        oldConfig.Networks.SubnetServices.Name,
        fldPath.Child("networks", "subnetServices", "name"))...)
}
```

Covers `C1`–`C12`, `D1`–`D4`.

### Runtime pre-flight validation

**File**: `pkg/controller/infrastructure/configvalidator.go`

The existing `ConfigValidator` already validates `CloudNAT.NatIPNames`. Extend `Validate()` with a BYO block that fires when `infra.IsUserManagedEgress()`:

```go
if infraConfig.IsUserManagedEgress() {
    // 1. VPC exists
    vpc, err := computeClient.GetNetwork(ctx, infraConfig.Networks.VPC.Name)
    if err != nil { return ... }
    if vpc == nil {
        allErrs = append(allErrs, field.Invalid(..., "referenced VPC does not exist"))
    }

    // 2. Worker subnet exists in region and belongs to the declared VPC
    sub, err := computeClient.GetSubnet(ctx, region, infraConfig.Networks.SubnetWorkers.Name)
    if err != nil { return ... }
    if sub == nil {
        allErrs = append(allErrs, field.Invalid(..., "referenced worker subnet does not exist"))
    }

    if sub != nil {
        // sub.Network is a self-link; compare the resource name suffix against VPC.Name
        if lastPathSegment(sub.Network) != infraConfig.Networks.VPC.Name {
            allErrs = append(allErrs, field.Invalid(..., "worker subnet does not belong to the referenced VPC"))
        }

        // 3. subnet CIDR contains shoot.spec.networking.nodes, non-overlapping with pods/services
        // subnetCIDR.ValidateSubset(nodes) — the nodes CIDR must fit inside the subnet

        // 4. Dual-stack checks
        if isDualStack {
            if sub.StackType != "IPV4_IPV6" {
                allErrs = append(allErrs, ...)
            }
            // secondary range named PodSecondaryRangeName exists and matches pods CIDR
            found := false
            for _, r := range sub.SecondaryIpRanges {
                if r.RangeName == *infraConfig.Networks.SubnetWorkers.PodSecondaryRangeName {
                    found = true
                    if r.IpCidrRange != podsCIDR {
                        allErrs = append(allErrs, ...)
                    }
                }
            }
            if !found {
                allErrs = append(allErrs, ...)
            }
        }
    }

    // 5. Services subnet (dual-stack): exists in region, belongs to declared VPC,
    //    stackType: IPV4_IPV6, IPv6 CIDR assigned
    if isDualStack && infraConfig.Networks.SubnetServices != nil {
        svcSub, err := computeClient.GetSubnet(ctx, region, infraConfig.Networks.SubnetServices.Name)
        if err != nil { return ... }
        if svcSub == nil {
            allErrs = append(allErrs, field.Invalid(..., "referenced services subnet does not exist"))
        }
        if svcSub != nil && lastPathSegment(svcSub.Network) != infraConfig.Networks.VPC.Name {
            allErrs = append(allErrs, field.Invalid(..., "services subnet does not belong to the referenced VPC"))
        }
        // verify svcSub.StackType == "IPV4_IPV6" and svcSub.Ipv6CidrRange != ""
    }
}
```

Covers `C13`–`C17`.

## Reconciler

### Helper function

**File**: `pkg/controller/infrastructure/infraflow/ensure_utils.go`

Add alongside the existing `isUserVPC` and `isUserRouter` functions:

```go
func isUserManagedEgress(config *gcp.InfrastructureConfig) bool {
    return config.IsUserManagedEgress()
}
```

### New ensure functions

**File**: `pkg/controller/infrastructure/infraflow/ensure.go`

Add two new functions adjacent to `ensureUserManagedVPC` and `ensureUserManagedCloudRouter`:

```go
// ensureUserManagedWorkersSubnet verifies the user's worker subnet exists, belongs to
// the configured VPC, and stores it on the whiteboard. Never creates or patches anything.
func (fctx *FlowContext) ensureUserManagedWorkersSubnet(ctx context.Context) error {
    subnetRef := fctx.config.Networks.SubnetWorkers
    if subnetRef == nil {
        return fmt.Errorf("subnetWorkers must not be nil in BYO mode")
    }
    subnet, err := fctx.computeClient.GetSubnet(ctx, fctx.infra.Spec.Region, subnetRef.Name)
    if err != nil {
        return err
    }
    if subnet == nil {
        return fmt.Errorf("user-managed worker subnet %q not found in region %q", subnetRef.Name, fctx.infra.Spec.Region)
    }
    vpc := GetObject[*compute.Network](fctx.whiteboard, ObjectKeyVPC)
    if subnet.Network != vpc.SelfLink {
        return fmt.Errorf("user-managed worker subnet %q belongs to network %q, not to the configured VPC %q", subnetRef.Name, subnet.Network, fctx.config.Networks.VPC.Name)
    }
    fctx.whiteboard.SetObject(ObjectKeyNodeSubnet, subnet)
    return nil
}

// ensureUserManagedServicesSubnet verifies the user's services subnet exists, belongs to
// the configured VPC, and stores it on the whiteboard. Never creates or patches anything.
func (fctx *FlowContext) ensureUserManagedServicesSubnet(ctx context.Context) error {
    subnetRef := fctx.config.Networks.SubnetServices
    if subnetRef == nil {
        return fmt.Errorf("subnetServices must not be nil for dual-stack BYO mode")
    }
    subnet, err := fctx.computeClient.GetSubnet(ctx, fctx.infra.Spec.Region, subnetRef.Name)
    if err != nil {
        return err
    }
    if subnet == nil {
        return fmt.Errorf("user-managed services subnet %q not found in region %q", subnetRef.Name, fctx.infra.Spec.Region)
    }
    vpc := GetObject[*compute.Network](fctx.whiteboard, ObjectKeyVPC)
    if subnet.Network != vpc.SelfLink {
        return fmt.Errorf("user-managed services subnet %q belongs to network %q, not to the configured VPC %q", subnetRef.Name, subnet.Network, fctx.config.Networks.VPC.Name)
    }
    fctx.whiteboard.SetObject(ObjectKeyServicesSubnet, subnet)
    return nil
}
```

### Task graph

**File**: `pkg/controller/infrastructure/infraflow/graph.go`

In the **reconcile graph**, replace the existing unconditional task registrations with conditional branches on `isUserManagedEgress(fctx.config)`:

```go
// Nodes subnet — branch on BYO mode
var ensureNodesSubnet *shared.Task
if isUserManagedEgress(fctx.config) {
    ensureNodesSubnet = fctx.AddTask(g, "ensure worker subnet",
        fctx.ensureUserManagedWorkersSubnet,
        shared.Timeout(defaultCreateTimeout),
        shared.Dependencies(ensureVPC))
} else {
    ensureNodesSubnet = fctx.AddTask(g, "ensure worker subnet",
        fctx.ensureNodesSubnet,
        shared.Timeout(defaultCreateTimeout),
        shared.Dependencies(ensureVPC, ensureDualStackKubernetesRoutesCleanup))
}

// Internal subnet — skip entirely in BYO mode (already handled by DoIf for managed mode)

// Services subnet — branch on BYO mode (dual-stack only)
if isDualStack && isUserManagedEgress(fctx.config) {
    fctx.AddTask(g, "ensure services subnet",
        fctx.ensureUserManagedServicesSubnet,
        shared.Timeout(defaultCreateTimeout),
        shared.Dependencies(ensureVPC))
} else if isDualStack {
    // existing ensureServicesSubnet task
}

// Cloud Router, Cloud NAT, IP addresses, firewall rules — skip in BYO mode
if !isUserManagedEgress(fctx.config) {
    // existing ensureCloudRouter, ensureCloudNAT, ensureIpAddresses, ensureFirewallRules tasks
}

// IPv6 CIDRs — skip in BYO mode (user's subnets already have them)
if isDualStack && !isUserManagedEgress(fctx.config) {
    // existing ensureIPv6CIDRs task
}
```

In the **delete graph**, add matching skip conditions:

```go
// Skip subnet, router, NAT, firewall deletion in BYO mode
fctx.AddTask(g, "destroy worker subnet",
    fctx.ensureSubnetDeletedFactory(...),
    shared.DoIf(!isUserManagedEgress(fctx.config)),
    ...)

fctx.AddTask(g, "ensure router deleted",
    fctx.ensureCloudRouterDeleted,
    shared.DoIf(!isUserManagedEgress(fctx.config) && !isUserRouter(fctx.config)),
    ...)

// ensureFirewallRulesDeleted still runs (cleans up CCM's k8s-fw-* rules) — no change needed
// ensureKubernetesRoutesDeleted still runs — no change needed
```

Covers `E1`–`E3` (reconciler makes no create/update calls against BYO resources).

### Status builder

**File**: `pkg/controller/infrastructure/infraflow/reconciler.go`

In the status-building section (around `buildStatus`), populate `Subnets[]` from the whiteboard in BYO mode:

```go
if isUserManagedEgress(fctx.config) {
    // PurposeNodes from whiteboard
    if sub, ok := fctx.whiteboard.GetObject(ObjectKeyNodeSubnet).(*compute.Subnetwork); ok && sub != nil {
        subnets = append(subnets, v1alpha1.Subnet{
            Name:    sub.Name,
            Purpose: v1alpha1.PurposeNodes,
            // CIDR is available from sub.IpCidrRange
        })
    }
    // PurposeServices (dual-stack)
    if isDualStack {
        if sub, ok := fctx.whiteboard.GetObject(ObjectKeyServicesSubnet).(*compute.Subnetwork); ok && sub != nil {
            subnets = append(subnets, v1alpha1.Subnet{
                Name:    sub.Name,
                Purpose: v1alpha1.PurposeServices,
            })
        }
    }
    // NatIPs remains empty; EgressCIDRs remains nil
}
```

Covers `E9`.

## Cloud-provider config

**File**: `pkg/controller/controlplane/valuesprovider.go`

In `getNetworkNames`, add an `infraConfig *apisgcp.InfrastructureConfig` parameter and change the nodes-subnet fallback so it only fires in BYO mode — not in managed mode when `Internal` happens to be absent:

```go
func getNetworkNames(
    infraStatus *apisgcp.InfrastructureStatus,
    infraConfig *apisgcp.InfrastructureConfig,
    cp *extensionsv1alpha1.ControlPlane,
) (string, string, string) {
    // ... existing networkName logic ...

    subNetworkName, subNetworkNameNodes := "", ""

    // Internal subnet (managed mode only)
    subnet, _ := apihelper.FindSubnetForPurpose(infraStatus.Networks.Subnets, apisgcp.PurposeInternal)
    if subnet != nil {
        subNetworkName = subnet.Name
    }

    // Nodes subnet
    subnet, _ = apihelper.FindSubnetForPurpose(infraStatus.Networks.Subnets, apisgcp.PurposeNodes)
    if subnet != nil {
        subNetworkNameNodes = subnet.Name
        // In BYO subnet mode there is no internal subnet; fall back to the nodes subnet
        // for internal LB frontend IPs. The guard is intentionally BYO-only: in managed
        // mode without an internal subnet, subNetworkName stays empty (existing behaviour).
        if infraConfig.IsUserManagedEgress() {
            subNetworkName = subnet.Name
        }
    }

    return networkName, subNetworkName, subNetworkNameNodes
}
```

No changes needed to the cloud-provider-config template itself — `subnetwork-name` is already conditionally emitted when `subNetworkName` is set.

Covers `E4`, `E6`.

## Bastion controller

**File**: `pkg/controller/bastion/actuator.go`

`getWorkersCIDR` currently reads from `InfrastructureConfig.Networks.Workers` (line ~77). In BYO mode that field is empty. Fix it to fall back to the CIDR from the `PurposeNodes` subnet in the infrastructure status:

```go
func getWorkersCIDR(cluster *controller.Cluster) (string, error) {
    infrastructureConfig := &apisgcp.InfrastructureConfig{}
    if err := json.Unmarshal(cluster.Shoot.Spec.Provider.InfrastructureConfig.Raw, infrastructureConfig); err != nil {
        return "", err
    }

    // BYO mode: Workers field is empty; read CIDR from InfrastructureStatus instead.
    if infrastructureConfig.IsUserManagedEgress() {
        infraStatus := &apisgcp.InfrastructureStatus{}
        if cluster.Shoot.Status.Provider == nil || cluster.Shoot.Status.Provider.InfrastructureStatus == nil {
            return "", fmt.Errorf("infrastructure status not available")
        }
        if err := json.Unmarshal(cluster.Shoot.Status.Provider.InfrastructureStatus.Raw, infraStatus); err != nil {
            return "", err
        }
        subnet, err := apihelper.FindSubnetForPurpose(infraStatus.Networks.Subnets, apisgcp.PurposeNodes)
        if err != nil || subnet == nil {
            return "", fmt.Errorf("could not find nodes subnet in infrastructure status")
        }
        return subnet.CIDR, nil
    }

    return infrastructureConfig.Networks.Workers, nil
}
```

Note: `Subnet.CIDR` may not be a field on the current status type — check `types_infrastructure.go`. If the CIDR is not stored in status today, it must be added when building the status from the BYO subnet's `IpCidrRange` (see the status builder section above).

Covers `E8`.

## Testing plan

### Unit tests

**`pkg/apis/gcp/validation/infrastructure_test.go`** — add a table-driven test block for BYO mode. Cover every case in `C1`–`C12` and `D1`–`D4`. Use the existing test helpers for building `InfrastructureConfig` objects.

**`pkg/controller/infrastructure/configvalidator_test.go`** — extend the existing `ConfigValidator` test with a BYO block. Mock `GetNetwork` and `GetSubnet` on `gcpComputeClient`. Cover:
- `C13`: subnet not found.
- `C14`: subnet CIDR does not contain the nodes CIDR (i.e. `subnetCIDR.ValidateSubset(nodes)` fails).
- `C15`: dual-stack subnet with `stackType: IPV4_ONLY`.
- `C16`: dual-stack subnet with missing secondary range.
- `C17`: dual-stack subnet with secondary range CIDR mismatch.
- Happy path for both IPv4 and dual-stack.

**`pkg/controller/infrastructure/infraflow/ensure_test.go`** (new or extend) — cover `ensureUserManagedWorkersSubnet`:
- Happy path: subnet found, stored on whiteboard.
- Subnet not found: error returned.
- Dual-stack path: `ensureUserManagedServicesSubnet` happy path and not-found error.

**`pkg/controller/controlplane/valuesprovider_test.go`** — add a test case for `getNetworkNames` in BYO mode:
- `PurposeInternal` absent, `PurposeNodes` present → `subNetworkName` equals nodes subnet name. Covers `E4`, `E6`.

**`pkg/controller/bastion/bastion_suite_test.go`** — extend `getWorkersCIDR` test with a BYO-mode scenario where `Networks.Workers` is empty and the CIDR is read from infrastructure status. Covers `E8`.

### Integration tests

**`test/integration/infrastructure/`** — extend the existing integration test harness to support a BYO-mode shoot variant. Pre-provisioning requirements:

- A VPC in the test GCP project.
- A worker subnet with a known primary CIDR that fits within the shoot's nodes CIDR.
- (Dual-stack variant) The worker subnet configured with `stackType: IPV4_IPV6`, a secondary range named `test-pods`, and an external IPv6 CIDR assigned; a services subnet with `stackType: IPV4_IPV6`.
- The four prerequisite firewall rules (two for IPv4, two for IPv6 in dual-stack) pre-created on the VPC.

New integration scenarios:
- `B1`: BYO subnet, IPv4, user-owned Cloud NAT → reconciles green; `cloudprovider.conf` has correct `subnetwork-name`.
- `B3`: BYO subnet, IPv4, no default route → reconciles green.
- `B4`: BYO subnet + services subnet, dual-stack → reconciles green.
- `E1`: verify no create/update calls were made against the BYO VPC or subnet (check GCP audit logs or assert no Gardener-named subnet exists in the VPC).
- `F1`: delete shoot → BYO VPC and subnet remain; no `k8s-fw-*` rules remain.

### Regression

`A1`–`A4` must continue to pass unchanged. Run the existing infrastructure integration suite against master then against this branch before merging.

## Suggested implementation order

Minimises risk by getting the machine-checkable parts (types, validation, unit tests) in first:

1. **API types + generated code** (`SubnetReference`, `SubnetWorkers`, `SubnetServices`, `IsUserManagedEgress`). No behavior change; unit tests can compile and be added.
2. **API-level validation** in `pkg/apis/gcp/validation/infrastructure.go`. Unit tests for `C1`–`C12`, `D1`–`D4`.
3. **Runtime pre-flight `ConfigValidator` extension**. Unit tests for `C13`–`C17`.
4. **Reconciler task-graph branching** — add `ensureUserManagedWorkersSubnet`, `ensureUserManagedServicesSubnet`, gate tasks in graph. Manual smoke test against a pre-provisioned BYO subnet in a scratch GCP project.
5. **Status-builder changes** — populate `Subnets[]` and (if not present) `CIDR` field from BYO subnet's `IpCidrRange`.
6. **`cloud-provider-config` fallback in `getNetworkNames`**. Unit test for `E4`, `E6`.
7. **Bastion `getWorkersCIDR` fix**. Unit test for `E8`.
8. **Integration test harness updates**. Add scenarios `B1`, `B3`, `B4`, `E1`, `F1`.
9. **Documentation** — `docs/usage/user-managed-egress.md` and the pointer from `docs/usage/usage.md` and `docs/usage/ipv6.md`.
