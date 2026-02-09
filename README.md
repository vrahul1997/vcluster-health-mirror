# health-mirror

`health-mirror` is a Kubernetes operator that provides **host-side visibility into vCluster health and sync coverage**.

It answers a simple but powerful question:

> _From the host cluster, which vClusters are running correctly and which sync subsystems are actually active?_

This project is designed for **platform engineers, SREs, and multi-tenant Kubernetes operators** who run multiple vClusters and want a lightweight, declarative way to monitor them.

---

## What problem does this solve?

When using **vCluster**, most signals live _inside_ the virtual cluster:

- Is the control plane running?
- Is DNS synced?
- Are nodes visible?
- Are workloads actually syncing to the host?

From the **host cluster**, these answers are not obvious.

`health-mirror` runs on the host cluster and continuously inspects Kubernetes resources to build a **Sync Coverage Index** for each vCluster.

---

## What does health-mirror detect?

For every discovered vCluster, the operator reports:

| Signal                | Meaning                                                    |
| --------------------- | ---------------------------------------------------------- |
| **ApiSync**           | vCluster API Service is discoverable on the host           |
| **ControlPlaneReady** | vCluster control-plane pod (`<name>-0`) is Running & Ready |
| **DnsSync**           | kube-dns mapping Service exists for the vCluster           |
| **NodeSync**          | virtual node mapping Services exist                        |
| **WorkloadSync**      | tenant workloads are synced and visible on the host        |

These signals are combined into:

- **Score** → percentage (0–100)
- **Level** → `None`, `Partial`, or `Full`

---

## Architecture (high level)

- Runs **only on the host cluster**
- Does **not** connect to vCluster APIs
- Uses Kubernetes-native discovery (Services, Pods, labels)
- No webhooks, no sidecars, no agents inside vClusters

This makes the operator:

- Safe
- Read-only
- Version-resilient

---

## Getting Started

### Prerequisites

- Kubernetes v1.25+
- vCluster installed on the host cluster
- Go v1.24+
- kubectl configured to talk to the **host cluster**

---

## Install the CRD

```bash
make install
```

---

## Run the controller locally (dev mode)

```bash
make run
```

This runs the controller against your current kubeconfig context.

---

## Create a VClusterHealth resource

Example:

```yaml
apiVersion: fleet.health.io/v1alpha1
kind: VClusterHealth
metadata:
    name: fleet
    namespace: default
spec:
    intervalSeconds: 30
    namespace: vcluster
```

Apply it:

```bash
kubectl apply -f config/samples/fleet_vclusterhealth.yaml
```

---

## View vCluster health

```bash
kubectl get vclusterhealth fleet -o yaml
```

Example output:

```yaml
syncCoverage:
    - clusterName: vc-prod
      apiSync: true
      controlPlaneReady: true
      dnsSync: true
      nodeSync: true
      workloadSync: true
      score: 100
      level: Full
```

---

## How workload sync is detected

`health-mirror` determines workload sync by observing **host-side pods** that:

- carry vCluster labels (e.g. `vcluster.loft.sh/managed-by=<vcluster>`), or
- follow vCluster namespace/name patterns

Control-plane pods are explicitly excluded.

No access to the vCluster API is required.

---

## Limitations (by design)

- This is **not** a runtime health checker (no readiness / liveness probing)
- It reports **sync enablement**, not application correctness
- Designed to be extended with custom signals if needed

---

## Roadmap

- Unit tests for all sync detectors
- Multi-namespace vCluster discovery
- Prometheus metrics export
- Optional runtime health signals

---

## Why this project exists

This project was built to demonstrate:

- Deep understanding of Kubernetes internals
- vCluster architecture and sync mechanics
- Operator design with testable, deterministic logic

It is intentionally simple, extensible, and production-minded.

---

## License

Apache 2.0

# vcluster-health-mirror

A small Kubernetes operator that gives you **host-side visibility into vCluster health and sync coverage**.

This project came out of a real operational problem:

> When running multiple vClusters, it’s hard to know from the **host cluster** whether they’re actually healthy — beyond just “the pod is running”.

`vcluster-health-mirror` answers that by observing Kubernetes-native signals and surfacing a **clear, opinionated health score** per vCluster.

No agents. No access to vCluster APIs. No magic.

---

## What does it do?

From the **host cluster**, the operator continuously discovers vClusters and reports:

- Is the vCluster API discoverable?
- Is the control plane actually _Ready_?
- Are DNS and virtual nodes synced?
- Are workloads syncing?
    - **system workloads** (CoreDNS, kube-system)
    - **tenant workloads** (your apps)

All of this rolls up into a **Score (0–100)** and a **Level** (`None`, `Partial`, `Full`).

---

## Signals detected

| Signal             | Meaning                                 |
| ------------------ | --------------------------------------- |
| ApiSync            | vCluster API Service exists on the host |
| ControlPlaneReady  | `<vcluster>-0` pod is Running & Ready   |
| DnsSync            | kube-dns mapping Service exists         |
| NodeSync           | virtual node mapping Services exist     |
| SystemWorkloadSync | kube-system pods are synced             |
| TenantWorkloadSync | non-kube-system workloads are synced    |

### Why split workloads?

A brand-new vCluster _should not_ look fully healthy.

- **SystemWorkloadSync** proves the vCluster infrastructure is alive
- **TenantWorkloadSync** proves users are actually running applications

This avoids false “green” states for empty clusters.

---

## Example output

```bash
kubectl get vclusterhealth
```

```text
NAME    TARGETNS   SCORE   LEVEL     SYSWL   TENANTWL   LASTUPDATED   AGE
fleet   *          83      Partial   true    false                   27h
```

Full detail:

```bash
kubectl get vclusterhealth fleet -o yaml
```

---

## How it works (high level)

- Runs **only on the host cluster**
- Discovers vClusters via `Service` objects with `app=vcluster`
- Evaluates health using:
    - Pods
    - Services
    - well-known vCluster labels
- Uses no webhooks, sidecars, or agents

This makes it:

- safe
- read-only
- resilient to vCluster upgrades

---

## Install & run

### Prerequisites

- Kubernetes v1.25+
- vCluster installed on the host cluster
- Go (for local development)

---

### Install CRDs

```bash
make generate
make manifests
make install
```

---

### Run controller locally (dev mode)

```bash
make run
```

Uses your current kubeconfig context.

---

## Create the Fleet resource

A ready-to-use example is provided:

```bash
kubectl apply -f fleet.yaml
```

`fleet.yaml`:

```yaml
apiVersion: fleet.health.io/v1alpha1
kind: VClusterHealth
metadata:
    name: fleet
    namespace: default
spec:
    intervalSeconds: 30
    namespace: '*' # discover vClusters across all namespaces
```

---

## Multi-namespace discovery

Configured via `spec.namespace`:

- `vcluster` → only that namespace
- `"*"` or `"all"` → discover vClusters across the entire cluster

---

## kubectl UX (PrintColumns)

The CRD defines custom columns so you can see health at a glance:

```bash
kubectl get vclusterhealth
```

No need to inspect full YAML for common checks.

---

## Testing

All health detectors and scoring logic are unit-tested.

```bash
go test ./... -v
```

Tests cover:

- control-plane readiness
- DNS sync detection
- node sync detection
- workload sync (system vs tenant)
- score and level calculation

---

## Design notes

- Health is derived from **first principles**, not internal APIs
- Signals are intentionally conservative
- Logic is deterministic and testable

This operator is small by design, but intentionally extensible.

---

## Why this project exists

This project was built to:

- deeply understand vCluster sync mechanics
- practice writing production-grade Kubernetes operators
- show how meaningful health can be inferred from core primitives

---

## License

Apache 2.0
