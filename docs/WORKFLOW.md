# Workflow

![workflow](../diagrams/workflow.svg)

Source: [`diagrams/workflow.d2`](../diagrams/workflow.d2).

## Phase 0 — Provisioning host

RHEL (or compatible) with:

- qemu-kvm, libvirt, virt-install, virsh
- dnsmasq and/or HAProxy (configs generated under `data/`)
- `oc`, `openshift-install` (matching target OCP version)
- pull secret + SSH public key via `.env`

## Phase 1 — Hub SNO

```bash
mini-acm hub create --config hub.yaml
```

1. Write `install-config.yaml` + `agent-config.yaml` under workDir.
2. `openshift-install agent create image`.
3. Ensure libvirt network; create SNO VM; attach ISO; boot.
4. Wait bootstrap + install-complete.
5. `mini-acm hub install-acm` → MCE + MultiClusterHub.

**OS truth:** control plane is **RHCOS**, provisioned by the Agent ISO. The provisioning host may be RHEL; the SNO node is not “RHEL + RPM OpenShift”.

## Phase 2 — Managed compact cluster

```bash
mini-acm cluster create --config cluster.yaml
```

1. Ensure network; emit HAProxy + dnsmasq fragments.
2. Create three master VMs (MACs/IPs reserved in config).
3. Emit `acm-resources.yaml` (ClusterDeployment, AgentClusterInstall, InfraEnv).
4. On hub: create namespace + pull-secret Secret; apply CRs; download discovery ISO.
5. `mini-acm cluster attach-iso --iso discovery.iso`.
6. Approve/bind Agents as control-plane; watch install; ManagedCluster import.

## Destroy

```bash
mini-acm cluster destroy --config cluster.yaml --yes
mini-acm hub destroy --config hub.yaml --yes [--purge]
```
