# MVP scope

**Version target: 0.1.0**

## In

1. Go CLI (`mini-mock`) with Cobra, Makefile, Containerfile (UBI), example configs.
2. `hub create` — local Agent-based Installer inputs + libvirt SNO VM + optional ACM manifests.
3. `cluster create` — isolated libvirt network hints, 3 VMs, HAProxy/dnsmasq templates, ACM CR YAML.
4. `--manual` / `--dry-run` escape hatches for every destructive virt step.
5. Profiles: hub-supported / hub-lab / supported / lab-small.
6. Provider interface with **libvirt** registered; stubs documented for later adapters.

## Out

- Automatic Agent approval / full unattended ISO download from hub (print steps).
- BMC / Redfish / BareMetalHost.
- demo.redhat.com API integration (mode stub only).
- Non-libvirt providers.

## Success checks

- [x] `make test` and `make build` pass
- [x] `make hub-dry` / `cluster-dry` print a coherent manual path
- [ ] On a KVM host with pull-secret: hub SNO reaches `install-complete`
- [ ] ACM MCH Available; compact Agents register and install progresses

## Manual path (always supported)

Every create command prints the next `openshift-install` / `oc` / `virsh` commands so a human can finish when automation stops short.
