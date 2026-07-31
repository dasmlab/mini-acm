# Test flow (read this first)

Prefer the **UI** for capturing parameters, then derive YAML for the CLI:

```bash
make build-all && make serve
# http://localhost:8080 → MockUps → Topology → Wizard → Derive
```

There are **two different bootable ISOs**. They are not the same artifact, and ACM is **not** baked into either ISO.

| Phase | Who builds the ISO | What boots | What you get |
|-------|--------------------|------------|--------------|
| **Hub** | Local `openshift-install agent create image` on the provisioner | 1× SNO VM | OpenShift SNO (RHCOS). **No ACM yet.** |
| **Managed cluster** | ACM **InfraEnv** on the hub (Assisted Service) | 3× VMs | Discovery → Agents → compact OCP install via ACM |

```text
[provisioner + libvirt]
        │
        │  hub create  ──► agent.x86_64.iso (local) ──► SNO OCP
        │                                                      │
        │  hub install-acm  ◄──────────────────────────────────┘
        │         │
        │         ▼
        │      MCE + ACM operators on hub
        │         │
        │  cluster create  ──► 3 VMs + net templates + CR YAML (no ISO yet)
        │         │
        │         ▼  (you apply CRs on hub)
        │      InfraEnv issues discovery ISO URL
        │         │
        │  attach-iso  ──► boot 3 VMs from discovery ISO ──► Agents ──► install
```

---

## 1. `hub create` — what is created?

**Not** “an OCP+ACM ISO”. Creates:

| Artifact | Unique per hub? | Source |
|----------|-----------------|--------|
| `install-config.yaml` | Yes (name, domain, network, pull secret, SSH) | Generated |
| `agent-config.yaml` | Yes (hostname, MAC, static IP, rendezvousIP) | Generated |
| `agent.x86_64.iso` | Yes | `openshift-install agent create image` |
| 1 libvirt VM | Yes (`node.hostname`) | Provider (libvirt) |
| libvirt network | Shared lab net (`provider.network`) | Provider |

**You must pass / set:**

| Input | How |
|-------|-----|
| Pull secret | `--pull-secret` or `$PULL_SECRET_FILE` |
| SSH public key | `--ssh-key` or `$SSH_PUBLIC_KEY_FILE` |
| Hub YAML | `--config` — see params below |

**Hub YAML — set these (no good defaults for identity/network):**

| Field | Example | Notes |
|-------|---------|--------|
| `metadata.name` | `hub` | Cluster name in install-config |
| `hub.baseDomain` | `lab.example.net` | DNS base |
| `hub.version` | `"4.18"` | Must match your `openshift-install` |
| `hub.mode` | `local-agent` | MVP path |
| `provider.type` | `libvirt` | Only real adapter today |
| `network.*` | CIDR / gateway / VIPs | Lab L3 |
| `node.hostname` / `ip` / `mac` | `hub-sno`, `.20`, MAC | Must match agent-config |

**Defaults from profile (omit to accept):**

| Profile | CPU | RAM | Disk |
|---------|-----|-----|------|
| `hub-supported` (default) | 8 | 24 GiB | 200 GiB |
| `hub-lab` | 8 | 16 GiB | 160 GiB |

**Manual test:**

```bash
cp .env.example .env   # real PULL_SECRET_FILE + SSH_PUBLIC_KEY_FILE
cp config/hub.example.yaml hub.yaml
# edit baseDomain / IPs / version to match your lab

make build
./bin/mini-acm --manual hub create --config hub.yaml --skip-wait --skip-acm
# follow printed openshift-install + virsh steps until:
#   export KUBECONFIG=./data/hub-hub/auth/kubeconfig
#   oc get nodes
```

---

## 2. `hub install-acm` — after hub OCP is up

Installs **operators on the live hub**, not a second ISO.

Applies (in order):

1. `manifests/acm/namespace.yaml` → `open-cluster-management`
2. `manifests/acm/mce-operator.yaml` → OperatorGroup + MCE Subscription + `MultiClusterEngine`
3. `manifests/acm/mch.yaml` → ACM Subscription + `MultiClusterHub`

| Knobs | Today |
|-------|--------|
| Channels (`stable-2.7`, `release-2.12`) | **Defaults in manifests** — edit YAML if your catalog differs |
| Namespace | Default `open-cluster-management` |
| Needs | Hub kubeconfig (`--kubeconfig` or workDir `auth/kubeconfig`) |

```bash
./bin/mini-acm hub install-acm --config hub.yaml --manual   # prints steps
# or:
./bin/mini-acm hub install-acm --config hub.yaml
oc get mch -n open-cluster-management -w   # wait for status Running / Available
```

Until MCE Assisted Service is healthy, **there is no discovery ISO** for managed clusters.

---

## 3. `cluster create` — what is created?

**Does not build an ISO.** Creates substrate + YAML you apply on the hub.

| Artifact | Unique? | Notes |
|----------|---------|--------|
| 3 libvirt VMs | Yes (`{name}-master-0..2`) | Empty/bootable later via discovery ISO |
| MACs / IPs | Yes from `ipBase` + `macPrefix` | Reserved in config |
| `haproxy.cfg` | Yes | API 6443, MCS 22623, ingress 80/443 → masters |
| `dnsmasq.d-*.conf` | Yes | `api` / `api-int` / `*.apps` |
| `acm-resources.yaml` | Yes | **Three** CRs (see below) — not a single “Cluster” CRD |

### ACM resources we generate (agent-based path)

Hive/ACM does **not** use one generic Cluster CRD for this path. MVP emits the standard trio:

| CRD | Name/NS | Unique fields we set | Defaults / placeholders |
|-----|---------|----------------------|-------------------------|
| **ClusterDeployment** | `{name}/{name}` | `clusterName`, `baseDomain`, `pullSecretRef`, `clusterInstallRef`, `platform.agentBareMetal` | `agentSelector: {}` |
| **AgentClusterInstall** | `{name}/{name}` | `controlPlaneAgents: 3`, `apiVIP`, `ingressVIP`, `machineNetwork`, `imageSetRef` | cluster/service CIDRs **hardcoded** `10.128.0.0/14`, `172.30.0.0/16`; SSH placeholder |
| **InfraEnv** | `{name}/{name}` | `clusterRef`, `pullSecretRef`, nmState label selector | SSH placeholder |

**Not generated yet (you may need manually for a clean install):**

| Resource | Why |
|----------|-----|
| **Namespace** `{name}` | Comment in YAML tells you to create it |
| **Secret** `pull-secret` | From `$PULL_SECRET_FILE` |
| **ClusterImageSet** | `imageSetRef.name` is derived as `img{versionCompact}-x86-64-appsub` (e.g. `4.18` → `img418-x86-64-appsub`) — must exist on hub or install stalls |
| **NMStateConfig** (×3) | Selector label is set; per-NIC YAML not emitted yet (DHCP may work on lab net; static preferred later) |
| **ManagedCluster** | Usually created/imported by ACM when install completes — not in our file |

**Adapter note:** `provider.type: libvirt` only affects **VM create** today. CRs always use `platform.agentBareMetal` (correct for Agent/InfraEnv). We are **not** multi-sizing per adapter in MVP — one compact shape per profile.

**Cluster YAML — set these:**

| Field | Example | Required |
|-------|---------|----------|
| `cluster.name` | `dev01` | Yes — drives NS + all CR names |
| `cluster.baseDomain` | `lab.example.net` | Yes |
| `cluster.version` | `"4.18"` | Yes — must match a ClusterImageSet |
| `cluster.profile` | `supported` \| `lab-small` | Profile fills CPU/RAM/disk |
| `provider.type` | `libvirt` | Yes |
| `network.machineCIDR` / `apiVIP` / `ingressVIP` / `gateway` | lab L3 | Yes |
| `nodes.ipBase` / `macPrefix` | `.21`, `52:54:00:13:00` | Yes |

**Defaults:** `nodes.count=3`, role `master`, sizes from profile.

```bash
cp config/cluster.example.yaml cluster.yaml
./bin/mini-acm --manual cluster create --config cluster.yaml

# On provisioner: install generated haproxy + dnsmasq fragments (point VIPs at gateway host)

# On hub:
export KUBECONFIG=./data/hub-hub/auth/kubeconfig
oc create namespace dev01
oc create secret generic pull-secret -n dev01 \
  --from-file=.dockerconfigjson=$PULL_SECRET_FILE \
  --type=kubernetes.io/dockerconfigjson
# Fix sshPublicKey placeholders in acm-resources.yaml, then:
oc apply -f data/cluster-dev01/acm-resources.yaml

# Ensure ClusterImageSet exists (name must match imageSetRef), then get ISO:
oc get infraenv -n dev01 -o jsonpath='{.items[0].status.isoDownloadURL}{"\n"}'
# curl/wget that URL → discovery.iso
```

---

## 4. `cluster attach-iso` — starts managed install

Attaches the **InfraEnv discovery ISO** to all three VMs and powers them on.

That is what starts host discovery → Agent CRs → (approve/bind) → Assisted install.

```bash
./bin/mini-acm cluster attach-iso --config cluster.yaml --iso ./discovery.iso
# then on hub:
oc get agents -n dev01 -w
# approve / set role master as needed for your ACM version
oc get agentclusterinstall -n dev01 -o yaml   # watch conditions
```

`attach-iso` does **not** create CRs or the ISO; it only boots machines into the ACM discovery flow.

---

## Ordered checklist (happy path)

1. [ ] Provisioner: libvirt + `oc` + `openshift-install` + `.env` secrets  
2. [ ] `hub create` → SNO `install-complete` → `oc get nodes`  
3. [ ] `hub install-acm` → MCH Available  
4. [ ] `cluster create` → 3 VMs + `data/cluster-*/` assets  
5. [ ] Apply DNS/HAProxy on provisioner  
6. [ ] On hub: ns + pull-secret + (ClusterImageSet) + `acm-resources.yaml`  
7. [ ] Download InfraEnv discovery ISO  
8. [ ] `attach-iso` → Agents appear → bind masters → install  
9. [ ] ManagedCluster imported; compact cluster API up via VIP  

## Destroy

```bash
./bin/mini-acm cluster destroy --config cluster.yaml --yes
# also: oc delete ns dev01  (or delete CRs) on hub
./bin/mini-acm hub destroy --config hub.yaml --yes --purge
```
