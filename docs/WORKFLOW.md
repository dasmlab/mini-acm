# Workflow

**Testing?** Start with [TEST-FLOW.md](TEST-FLOW.md) — two ISOs, CRD table, checklist.

![workflow](../diagrams/workflow.svg)

Source: [`diagrams/workflow.d2`](../diagrams/workflow.d2).

## Mental model

| Command | Creates ISO? | Creates |
|---------|--------------|---------|
| `hub create` | **Yes** — local Agent ISO (`openshift-install`) | SNO OCP only (RHCOS). Not ACM. |
| `hub install-acm` | No | MCE + ACM operators on the live hub |
| `cluster create` | **No** | 3 VMs + DNS/HAProxy templates + ACM CR YAML |
| `cluster attach-iso` | No (you pass ISO) | Boots VMs from **InfraEnv discovery ISO** → Agents → install |

ACM never lives inside the hub Agent ISO. Managed-cluster media comes from InfraEnv **after** ACM is up and CRs are applied.

## Phase 0 — Provisioning host

RHEL (or compatible) with:

- qemu-kvm, libvirt, virt-install, virsh
- dnsmasq and/or HAProxy (configs generated under `data/`)
- `oc`, `openshift-install` (matching target OCP version)
- pull secret + SSH public key via `.env`

## Phase 1 — Hub SNO

```bash
mock-me hub create --config hub.yaml
```

1. Write `install-config.yaml` + `agent-config.yaml` under workDir.
2. `openshift-install agent create image` → `agent.x86_64.iso`.
3. Ensure libvirt network; create SNO VM; attach ISO; boot.
4. Wait bootstrap + install-complete.
5. `mock-me hub install-acm` → MCE + MultiClusterHub.

**OS truth:** control plane is **RHCOS**, provisioned by the Agent ISO. The provisioning host may be RHEL; the SNO node is not “RHEL + RPM OpenShift”.

## Phase 2 — Managed compact cluster

```bash
mock-me cluster create --config cluster.yaml
```

1. Ensure network; emit HAProxy + dnsmasq fragments.
2. Create three master VMs (MACs/IPs reserved in config).
3. Emit `acm-resources.yaml`: **ClusterDeployment** + **AgentClusterInstall** + **InfraEnv** (agent-based path — not one generic Cluster CRD).
4. On hub: create ns + pull-secret (+ ClusterImageSet if missing); apply CRs; **download discovery ISO** from InfraEnv status.
5. `mock-me cluster attach-iso --iso discovery.iso`.
6. Approve/bind Agents as control-plane; watch install; ManagedCluster import.

`provider.type` selects the VM adapter (libvirt). ACM CRs stay `agentBareMetal` for this lifecycle. One size profile per cluster in MVP (no per-adapter sizing matrix yet).

## Destroy

```bash
mock-me cluster destroy --config cluster.yaml --yes
mock-me hub destroy --config hub.yaml --yes [--purge]
```
