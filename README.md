# vcluster-health-mirror

`vcluster-health-mirror` is a Kubernetes operator that gives you **host-side visibility into vCluster health and sync coverage**.

It answers a simple question:

> From the host cluster, which vClusters are actually working — and which sync subsystems are really active?

It’s intentionally lightweight:

- runs **only on the host cluster**
- **read-only** (observes Services/Pods/labels)
- no agents, no webhooks, no vCluster API access required

---

## What it detects

For every discovered vCluster, the operator reports these signals:

| Signal                 | Meaning                                          |
| ---------------------- | ------------------------------------------------ |
| **ApiSync**            | vCluster API Service is discoverable on the host |
| **ControlPlaneReady**  | `<vcluster>-0` pod is Running & Ready            |
| **DnsSync**            | kube-dns mapping Service exists                  |
| **NodeSync**           | virtual node mapping Services exist              |
| **SystemWorkloadSync** | kube-system workloads are synced (e.g. CoreDNS)  |
| **TenantWorkloadSync** | non-kube-system workloads are synced (your apps) |

These roll up into:

- **Score** (0–100)
- **Level**: `None`, `Partial`, `Full`

### Why split workloads?

A brand-new vCluster should _not_ look fully healthy.

- **SystemWorkloadSync** proves the infra is alive
- **TenantWorkloadSync** proves users are actually running apps

This avoids false “green” states for empty clusters.

---

## Quick demo

Once installed, apply the Fleet resource and watch it populate:

```bash
kubectl apply -f fleet.yaml
kubectl get vclusterhealth
kubectl get vclusterhealth fleet -o yaml
```

The CRD also defines **PrintColumns**, so you can see Score/Level directly in `kubectl get vclusterhealth`.

---

## Install (kubectl apply)

This is the simplest way to try the operator.

From this repo, we ship:

- `dist/install.yaml` (CRDs + RBAC + controller Deployment)
- `fleet.yaml` (a ready-to-use Fleet resource)

### Install the operator

```bash
kubectl apply -f dist/install.yaml
```

### Create the Fleet resource

```bash
kubectl apply -f fleet.yaml
```

### Verify

```bash
kubectl get pods -A | grep -i health-mirror || true
kubectl get vclusterhealth
```

> Tip: If you attach `dist/install.yaml` to a GitHub Release, users can install directly from the release URL.

---

## Install (Helm)

If you prefer Helm lifecycle management, the chart lives at `./charts/health-mirror`.

```bash
helm upgrade --install health-mirror ./charts/health-mirror \
  --namespace health-mirror --create-namespace \
  --set image.repository=ghcr.io/<YOUR_GITHUB_USERNAME>/vcluster-health-mirror \
  --set image.tag=v0.1.0 \
  --set fleet.create=true \
  --set fleet.namespaceSelector="*" \
  --set fleet.intervalSeconds=30
```

---

## Multi-namespace discovery

Configured via the Fleet spec:

- `namespace: vcluster` → only that namespace
- `namespace: "*"` or `"all"` → discover vClusters across the entire host cluster

---

## Testing

Unit tests cover the sync detectors and scoring logic:

```bash
go test ./... -v
```

---

## Building the release artifacts

### Build and push the controller image

```bash
export IMG=ghcr.io/<YOUR_GITHUB_USERNAME>/vcluster-health-mirror:v0.1.0
make docker-build IMG=$IMG DOCKER_BUILDKIT=1
make docker-push IMG=$IMG
```

### Generate a single install.yaml

```bash
make build-installer IMG=$IMG
```

This produces `dist/install.yaml`.

---

## License

Apache 2.0
