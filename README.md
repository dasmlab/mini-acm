# mini-acm

> ================================================================================
>
> ## LAB / TEST / DEV ONLY
>
> This tool builds a **virtual rack** for exercising ACM cluster lifecycle
> (InfraEnv / Agents / compact OCP) on cheap lab VMs. It is **not** a
> supported production OpenShift or ACM installer.
>
> Secrets (pull secret, SSH keys, kubeconfigs) live in **env / files only** —
> never in committed YAML.
>
> ================================================================================

**Version: 0.1.0 (MVP)** · [**Test flow**](docs/TEST-FLOW.md) · [Workflow](docs/WORKFLOW.md) · [MVP](docs/MVP.md) · [Roadmap](docs/ROADMAP.md) · [Diagram](diagrams/workflow.d2)

## What this is

`mini-acm` is a Go CLI that:

1. **Bootstraps a small management SNO** with the **local Agent-based Installer** (`openshift-install agent`) — RHCOS, not “RHEL then install OCP”.
2. **Installs MCE + ACM** on that hub (Operator manifests).
3. **Presents three libvirt VMs** on one lab network and generates ACM CR templates + DNS/HAProxy so ACM can install a **compact 3-node** managed cluster.

Provider adapters own machines. ACM owns OpenShift lifecycle.

```text
ocp-lab create path (MVP commands)
  mini-acm hub create      → SNO + optional ACM
  mini-acm cluster create  → 3 VMs + ACM CR / net assets
```

## Quick start (RHEL INFRA-HOST — intended UX)

1. Commission **RHEL 9/10**, activate subscription (or bring your own packages / fork).
2. Clone this repo on that host (BM or nested-virt VM).
3. Run the **container runtime** with libvirt access; author in UI or CLI.
4. **Deploy** — runtime applies the MockUp (VMs on that host). Long-term: Ansible playbooks in the same EE the UI signals; today: derive YAML + `mini-acm` commands / `--manual`.

See [docs/ROADMAP.md](docs/ROADMAP.md) for the EE / Ansible / as-a-service adapter model.

## Quick start (CLI)

```bash
cp .env.example .env          # set PULL_SECRET_FILE + SSH_PUBLIC_KEY_FILE
cp config/hub.example.yaml hub.yaml
cp config/cluster.example.yaml cluster.yaml

make build
./bin/mini-acm --manual hub create --config hub.yaml --skip-wait
```

## Quick start (UI — MockUp topology)

Same pattern as etcd-synthetic-load / interview-me: Vue+Quasar SPA embedded in `serve`.

**Prod (2026-prod-1):** https://mini-acm.apps.2026-prod-1.ocp.dasmlab.org  
(Argo app `mini-acm` → `mini-acm-system`; GHCR `ghcr.io/dasmlab/mini-acm`; HAProxy `CERT53`)

```bash
make build-all          # npm build UI → embed → go binary
./bin/mini-acm serve --listen :8080 --data-dir ./data
# open http://localhost:8080
```

Flow in the UI:

1. **MockUps** — create a top-level lab rack (Target analogue)
2. **Topology** — drag MGMT-CLUSTER / ACM / DEPLOYMENT-CLUSTER nodes; click to edit (CPU/RAM/disk sliders)
3. **Wizard** — capture MVP-gap params (pull-secret path, ClusterImageSet, discovery ISO, …)
4. **Derive** — write `data/mockups/<id>/out/hub.yaml` + `cluster-*.yaml` for the CLI

### Container

```bash
podman build -t mini-acm -f Containerfile .
podman run --rm -v "$PWD/data:/data:Z" --env-file .env mini-acm --help
```

Libvirt/ISO attach still needs a host with KVM (or a future remote provider).

## Profiles

| Profile | Topology | Per node | Notes |
|---------|----------|----------|--------|
| `hub-supported` | SNO | 8 vCPU / 24 GiB / 200 GiB | Default hub (room for ACM) |
| `hub-lab` | SNO | 8 / 16 / 160 | Unsupported squeeze |
| `supported` | compact 3 | 4 / 16 / 120 | Default managed |
| `lab-small` | compact 3 | 4 / 12 / 120 | Unsupported |

Do not default to 8 GiB nodes — that turns the exercise into operator failure debugging.

## Non-goals (MVP)

- BareMetalHost / virtual BMC / Metal3
- Cloud adapters (Azure/ARO-like, VMware, …) — interface only
- Hosted Control Planes / KubeVirt
- demo.redhat.com end-to-end (stub `hub.mode: demo-redhat`)
- Shipping pull secrets or kubeconfigs in git

## Repository layout

```text
cmd/mini-acm/          cobra CLI (+ embedded static/)
web/                   Vue 3 + Quasar UI (MockUps / Topology / Wizard)
internal/
  config/              hub + cluster YAML
  mockup/              MockUp store (Target analogue)
  api/                 chi /api/v1 + SPA static
  provider/            Provider interface + libvirt
  hub/                 Agent-based SNO bootstrap
  cluster/             compact cluster orchestration
  acm/                 MCE/ACM apply helpers
  netcfg/              HAProxy + dnsmasq fragments
config/*.example.yaml
profiles/
manifests/acm/
docs/
diagrams/
```

## Provider interface

```text
CreateNode DeleteNode AttachISO DetachISO
StartNode StopNode GetMAC GetPowerState ListNodes EnsureNetwork
```

MVP implements **libvirt**. Register more under `internal/provider/<name>`.

## Auth / secrets

| Var | Purpose |
|-----|---------|
| `PULL_SECRET_FILE` / `PULL_SECRET` | Red Hat pull secret |
| `SSH_PUBLIC_KEY_FILE` / `SSH_PUBLIC_KEY` | Node SSH key |
| `KUBECONFIG` / `--kubeconfig` | Hub API access for ACM apply |

## Make targets

| Target | What |
|--------|------|
| `make build` | `./bin/mini-acm` |
| `make build-all` | Vue UI embed + Go binary |
| `make serve` | UI+API on :8080 |
| `make test` | `go vet` + `go test` |
| `make hub-dry` / `cluster-dry` | Manual/dry-run example configs |
| `make image` | UBI9 image via podman |

## License

Lab tooling for DASMLAB — no production support implied.
