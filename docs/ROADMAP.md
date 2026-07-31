# Roadmap

## Done (0.1 MVP)

- Hub local-agent path + ACM manifest bundle
- Compact cluster asset generation + libvirt provider
- Manual/dry-run modes, profiles, docs/diagrams

## Next

1. **Unattended ISO fetch** from InfraEnv download URL via hub kubeconfig.
2. **Agent wait/approve/bind** helpers (`oc`/`client-go`).
3. **ImageSet** ClusterImageSet apply for the configured OCP version.
4. **NMStateConfig** CRs generated per MAC/IP from cluster config.
5. Harden libvirt AttachISO (correct cdrom device discovery).

## Later adapters

| Provider | Notes |
|----------|--------|
| Proxmox | Same Provider interface |
| VMware | govmomi |
| Azure / ARO-like | VMs + LB (conceptual cousin to ARO, not AKS) |
| baremetal-redfish | sushy-tools / virtual BMC — Metal3 path |
| KubeVirt / HCP | Different lifecycle (HostedCluster), optional cluster type |
| **demo.redhat.com** | OTT API with creds to order precanned demo; reuse status UX |

## Explicit non-goals unless requested

- Replacing ACM Assisted Service with a fake Kubernetes
- Cloning finished SNO disks as “golden images” (cluster identity collision)
- Documented support for 8 GiB control-plane nodes
