# Roadmap

## Done (0.1 MVP)

- Hub local-agent path + ACM manifest bundle
- Compact cluster asset generation + libvirt provider
- Manual/dry-run modes, profiles, docs/diagrams
- MockUp UI: Topology + Wizard; multi DEPLOYMENT-CLUSTER; INFRA-HOST object
- Prod GitOps on 2026-prod-1 (UI-as-a-service path)

## Target MockUp object model

```text
MockUp (canvas)
├── MACHINE-HOST     RHEL 9/10 where libvirtd runs (BM or nested). SSH e.g. 192.168.1.142
│                    disks: system (OS/logs) + pool (guest images)
│                    nics: bridged uplink + optional host-only (VMnet12 — future class move)
├── VYOS-GW          Edge router VM on MACHINE-HOST
│                    eth0 WAN = host bridge · eth1 LAN = libvirt net (obscure CIDR 10.77.30.0/24)
│                    NAT/FW — installer later; object is inventory now
├── MGMT-CLUSTER     SNO guest on LAN (governance OCP)
├── ACM              Operators on MGMT-CLUSTER
└── DEPLOYMENT-CLUSTER×N   Compact guest sets on LAN; ACM owns LC
```

Lab guests never sit on VMnet12; they sit on the VyOS LAN libvirt network.

## Topology views (UI)

| View | Shows | Point |
|------|-------|--------|
| **Infrastructure** | MACHINE-HOST → ADAPTER → **vHosts** · **VyOS** on GW vHost | Guests: 1× GW, 1× MGMT (SNO), **3× cp/worker** per deployment. |
| **Network** | PRI-PHY-NIC · VyOS eth0 WAN / eth1 LAN (.1) · **vSwitch** · guest vNICs | Adapter’s libvirt LAN picture; WAN ties RTR to host primary NIC. |
| **Cluster mgmt** | ACM + home mgmt OCP + managed deployments | OCP objects. ACM lives on mgmt; governs spokes. Not full self-mgmt yet. |
| **Application** | ACM | Payload today; Ansible / GitOps / test apps can land on clusters later. |
| **Full rack** | High-level bands | Detailed NICs live in Network tab. |

**Later:** HAProxy on the cluster path; **arbiter** topology (2cp+worker + tiny arbiter vHost/stack); ACM self-management / sovereign; free-form → guided promote.

### Free-form / creative canvas (teaching)

- Toggle **Guided** vs **Free-form** on Topology.
- Free-form: no constrained relation edges by default; drop orphan **vHosts**, **HAProxy** / **other** appliances; hide rack pieces; **Validate** collects missing payload / mgmt / ACM / spoke errors in one pass.
- **Not supported (stub):** promoting a free-form canvas into a guided/constrained MockUp — rebuild in Guided if you want derive/deploy. Track as later improvement.

The “happy path” is **one subscribed RHEL 9/10 machine** (BM or nested-virt VM) that is both the **INFRA-HOST** and where mini-acm’s container runtime talks to libvirt:

```text
1. Commission RHEL 9/10 + activate subscription
   (without subscription you bring your own packages / fork)
2. Clone this repo
3. Run the container runtime (podman) with host libvirt access
   — smoke: API up, qemu/libvirt reachable, capacity matches MockUp
4. Author in UI (MockUp → Topology → Wizard) or CLI / derive YAML
5. DEPLOY — runtime applies the plan (VMs + net + ACM assets)
```

**Deploy executor (chosen direction):** Ansible playbooks inside a **singleton execution environment** (same image the UI signals), not ad-hoc shell from the Go binary forever.

| Mode | Who runs the EE | Infra |
|------|-----------------|--------|
| **Singleton (lab)** | podman on the RHEL INFRA-HOST | local libvirt socket / qemu |
| **As-a-service** | EE on cluster (Controller/EDA-style job) | provider **adapter** points at a RHEL host (or equivalent) that meets topology capacity |

UI → core is the same signal either way (MockUp id / derived artifacts / “deploy” action). When to split UI and core into separate images is deferred; keep one EE until job boundaries hurt.

```text
┌─────────────────────────────────────────────────────────┐
│  RHEL INFRA-HOST (BM or nested)                         │
│  subscription · kvm · libvirt · podman                  │
│                                                         │
│   ┌─────────────────── EE / runtime ─────────────────┐  │
│   │  mini-acm serve (UI+API)                         │  │
│   │  deploy job ──► Ansible playbooks                │  │
│   │       · ensure libvirt net / pool                │  │
│   │       · create hub + cluster VMs                 │  │
│   │       · DNS/HAProxy fragments                    │  │
│   │  Go helpers ──► client-go / openshift libs       │  │
│   │       · ClusterDeployment / InfraEnv / Agents    │  │
│   │       · MCE/ACM operators, waits, ImageSets      │  │
│   └──────────────────────────────────────────────────┘  │
│          │ virsh / qemu                                  │
│          ▼                                               │
│   guest VMs: MGMT-CLUSTER + N× DEPLOYMENT-CLUSTER        │
└─────────────────────────────────────────────────────────┘
```

**Division of labor**

| Layer | Owns |
|-------|------|
| **INFRA-HOST** (topology object) | RHEL machine facts, capacity, libvirt URI, subscription expectation |
| **Ansible EE** | Idempotent host + VM lifecycle (RHEL packages, libvirt, guests, net) |
| **Go CLI / libs** | MockUp store, derive YAML, OCP/ACM API (k8s + OpenShift clients), `--manual` escape |
| **Provider adapters** | Where VMs live: local libvirt today; later remote host / cloud / cluster-pointed infra |
| **ACM on hub** | Real OpenShift lifecycle (InfraEnv, Agents, ClusterDeployment, …) |

Go `provider/libvirt` remains useful as a thin driver *or* as what playbooks wrap; Ansible is the **user-facing deploy path** on RHEL so the same EE can be invoked from UI, CLI, or AAP/EDA later.

## Next (near-term)

0. **Inventory (MACHINE-HOST targets)** — DONE MVP: seed `dasm@192.168.1.142`, CRUD, SSH probe (auth + libvirt readiness). Orchestrate/deploy against a plan is next.
1. **Host bootstrap doc + script** — RHEL 9/10: subscription, `libvirt`/`qemu-kvm`/`podman`, nested-virt note, socket permissions for the container.
2. **Container ↔ libvirt** — documented `podman run` (volume `/var/run/libvirt`, device, or ssh+bastion); smoke via Inventory **Probe** + later `GET /api/v1/infra/health`.
3. **Deploy signal** — `POST …/mockups/{id}/deploy` queues a job against linked Inventory host; MVP may call Ansible locally or shell out to existing `hub create` / `cluster create` behind a job status UX.
4. **Ansible skeleton** — `ansible/` playbooks consuming `out/infra-host.yaml` + hub/cluster YAML; EE Collection-friendly layout.
5. Keep existing MVP hardening: InfraEnv ISO fetch, Agent approve/bind, ClusterImageSet, NMStateConfig, AttachISO.

## Later adapters

| Provider | Notes |
|----------|--------|
| libvirt (local) | Default on INFRA-HOST |
| libvirt-remote | Same playbooks, adapter URI to another RHEL host |
| Proxmox / VMware / Azure | Provider interface; playbooks or Go per adapter |
| baremetal-redfish | sushy-tools / virtual BMC — Metal3 path |
| KubeVirt / HCP | Different lifecycle (HostedCluster) |
| **demo.redhat.com** | OTT API; reuse status UX |
| **AAP / EDA** | Same EE image as Controller execution environment |

## Explicit non-goals unless requested

- Replacing ACM Assisted Service with a fake Kubernetes
- Cloning finished SNO disks as “golden images” (cluster identity collision)
- Documented support for 8 GiB control-plane nodes
- Shipping pull secrets or activated subscription credentials in git
