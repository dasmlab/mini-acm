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

**Version: 0.1.0 (MVP)** · [Workflow](docs/WORKFLOW.md) · [MVP](docs/MVP.md) · [Roadmap](docs/ROADMAP.md) · [Diagram](diagrams/workflow.d2)

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

## Quick start

```bash
cp .env.example .env          # set PULL_SECRET_FILE + SSH_PUBLIC_KEY_FILE
cp config/hub.example.yaml hub.yaml
cp config/cluster.example.yaml cluster.yaml

make build
./bin/mini-acm --manual hub create --config hub.yaml --skip-wait
# follow printed openshift-install / virsh steps, then:
./bin/mini-acm hub install-acm --config hub.yaml --manual

./bin/mini-acm --manual cluster create --config cluster.yaml
# apply data/cluster-*/acm-resources.yaml on hub, download discovery ISO, then:
./bin/mini-acm cluster attach-iso --config cluster.yaml --iso ./discovery.iso --manual
```

Dry-run without touching libvirt:

```bash
make hub-dry
make cluster-dry
```

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
cmd/mini-acm/          cobra CLI
internal/
  config/              hub + cluster YAML
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
| `make test` | `go vet` + `go test` |
| `make hub-dry` / `cluster-dry` | Manual/dry-run example configs |
| `make image` | UBI9 image via podman |

## License

Lab tooling for DASMLAB — no production support implied.
