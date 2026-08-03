package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dasmlab/mock-me/internal/mockup"
)

func (e *Engine) stageGenerate(j *Job) error {
	m, err := e.mockups.Get(j.MockUpID)
	if err != nil {
		return err
	}
	localDir := e.mockups.Dir(j.MockUpID)
	e.log(j, StageGenerate, "local mockup dir %s", localDir)
	_ = e.SaveJob(j)

	paths, err := e.mockups.Derive(j.MockUpID)
	if err != nil {
		return fmt.Errorf("derive: %w", err)
	}
	m, _ = e.mockups.Get(j.MockUpID)

	remoteRoot := remoteWorkRoot(m.Metadata.Name)
	e.log(j, StageGenerate, "remote work root %s", remoteRoot)
	_ = e.SaveJob(j)

	var copied []string
	var logLines []string
	for key, local := range paths {
		b, err := os.ReadFile(local)
		if err != nil {
			return fmt.Errorf("read %s: %w", local, err)
		}
		remote := remoteRoot + "/out/" + filepath.Base(local)
		e.log(j, StageGenerate, "copy %s  %s → %s (%d bytes)", key, local, remote, len(b))
		_ = e.SaveJob(j)
		if err := e.inv.WriteRemoteFile(j.InventoryID, remote, b); err != nil {
			return err
		}
		copied = append(copied, key+"→"+filepath.Base(local))
		logLines = append(logLines, fmt.Sprintf("%s\n  local:  %s\n  remote: %s", key, local, remote))
	}

	planRemote := remoteRoot + "/PLAN.txt"
	index := fmt.Sprintf("mockup=%s\nstyle=%s\nhost=%s\nartifacts=%s\n",
		m.Metadata.Name, m.Spec.Style, j.HostEndpoint, strings.Join(copied, ","))
	e.log(j, StageGenerate, "write index %s", planRemote)
	_ = e.SaveJob(j)
	if err := e.inv.WriteRemoteFile(j.InventoryID, planRemote, []byte(index)); err != nil {
		return err
	}

	e.setStage(j, StageGenerate, StageOK,
		fmt.Sprintf("Wrote %d plan files to %s/out on %s", len(copied), remoteRoot, j.HostName),
		strings.Join(logLines, "\n\n"))
	return nil
}

func (e *Engine) stageEE(j *Job) error {
	img := EEImage()
	e.log(j, StageEE, "ensure curated EE %s on %s (host needs podman only)", img, j.HostEndpoint)
	_ = e.SaveJob(j)
	script := "set -eu\necho EE_CHECK_START\n" + eeEnsureScript()
	out, _, err := e.inv.RunScript(j.InventoryID, script)
	if err != nil {
		return fmt.Errorf("execution env: %v (%s)", err, truncate(out, 500))
	}
	e.log(j, StageEE, "host stdout:\n%s", trimHostOut(out))
	_ = e.SaveJob(j)
	if strings.Contains(out, "EE_FAIL=podman") {
		return blocked("podman not on inventory host — required to run the curated mock-me EE; Probe → Fix this (install-podman), then Clean + Deploy")
	}
	if strings.Contains(out, "EE_FAIL=ee-image") {
		return blocked(fmt.Sprintf("curated EE image %s not pullable — Probe → Fix this (ensure-mock-me-ee), or set MOCK_ME_EE_IMAGE; then Clean + Deploy", img))
	}
	if strings.Contains(out, "EE_FAIL=ee-tools") {
		return blocked(fmt.Sprintf("EE image %s is present but missing openshift-install/oc — rebuild/push mock-me-ee, then Fix this / re-Deploy", img))
	}
	if !strings.Contains(out, "EE_OK=1") {
		return fmt.Errorf("EE check incomplete: %s", truncate(out, 500))
	}
	e.setStage(j, StageEE, StageOK,
		fmt.Sprintf("Curated EE ready (%s) — openshift-install + oc in container; host only needed podman", img),
		out)
	_ = e.SaveJob(j)
	return nil
}

func (e *Engine) stageVInfra(j *Job) error {
	m, err := e.mockups.Get(j.MockUpID)
	if err != nil {
		return err
	}
	netName := m.Spec.InfraHost.NetworkName
	if netName == "" {
		netName = "ocp-lab"
	}
	pool := m.Spec.InfraHost.StoragePool
	if pool == "" || pool == "default" {
		pool = HostPoolName
	}
	cidr := m.Spec.Network.MachineCIDR
	if cidr == "" {
		cidr = "10.77.30.0/24"
	}
	gw := m.Spec.Network.Gateway
	if gw == "" {
		gw = "10.77.30.1"
	}
	guests := buildGuests(m)
	e.log(j, StageVInfra, "host data root %s — libvirt pool=%q net=%q cidr=%s — materialize %d guest(s)",
		HostDataRoot, pool, netName, cidr, len(guests))
	for _, g := range guests {
		e.log(j, StageVInfra, "plan guest %s role=%s cpu=%d memMiB=%d diskGiB=%d mac=%s",
			g.Name, g.Role, g.CPU, g.MemoryMiB, g.DiskGiB, g.MAC)
	}
	_ = e.SaveJob(j)

	script := ensureHostLayoutScript(pool) + fmt.Sprintf(`
echo VINFRA_START
export LIBVIRT_DEFAULT_URI="${LIBVIRT_DEFAULT_URI:-qemu:///system}"
# RHEL 10 uses modular virtqemud — libvirtd.service may be inactive; do not require sudo.
if ! virsh list >/dev/null 2>&1; then
  systemctl --user start virtqemud.socket 2>/dev/null || true
  sudo -n systemctl start virtqemud.socket 2>/dev/null || true
  sudo -n systemctl start libvirtd 2>/dev/null || true
fi
virsh list >/dev/null
command -v virt-install >/dev/null || { echo "virt-install missing — Fix this install-libvirt"; exit 1; }
NET=%q
GW=%q
HUB_IP=%q
CLUSTER=%q
BASE_DOMAIN=%q
API_HOST="api.${CLUSTER}.${BASE_DOMAIN}"
API_INT_HOST="api-int.${CLUSTER}.${BASE_DOMAIN}"
if ! virsh net-info "$NET" >/dev/null 2>&1; then
  cat > /tmp/mock-me-net.xml <<EOF
<network>
  <name>$NET</name>
  <forward mode='nat'/>
  <bridge name='virbr-mm' stp='on' delay='0'/>
  <dns>
    <host ip='$HUB_IP'>
      <hostname>$API_HOST</hostname>
      <hostname>$API_INT_HOST</hostname>
    </host>
  </dns>
  <ip address='$GW' netmask='255.255.255.0'>
    <dhcp>
      <range start='10.77.30.100' end='10.77.30.200'/>
    </dhcp>
  </ip>
</network>
EOF
  virsh net-define /tmp/mock-me-net.xml
  virsh net-start "$NET" || true
  virsh net-autostart "$NET" || true
fi
# Ensure API DNS on existing nets (required for bootstrap / oc against kubeconfig).
virsh net-update "$NET" delete dns-host "<host ip='$HUB_IP'></host>" --live --config 2>/dev/null || true
virsh net-update "$NET" add dns-host "<host ip='$HUB_IP'><hostname>$API_HOST</hostname><hostname>$API_INT_HOST</hostname></host>" --live --config 2>/dev/null \
  || virsh net-update "$NET" add-last dns-host "<host ip='$HUB_IP'><hostname>$API_HOST</hostname><hostname>$API_INT_HOST</hostname></host>" --live --config 2>/dev/null \
  || true
# Seed-side resolution for podman oc (no sudo): file consumed by OCP wait --add-host.
mkdir -p /vm-disks/mock-me/work
printf '%%s %%s %%s\n' "$HUB_IP" "$API_HOST" "$API_INT_HOST" > "/vm-disks/mock-me/work/${CLUSTER}-api-hosts" 2>/dev/null || true
echo "NET_INFO<<"
virsh net-info "$NET" || true
virsh net-dumpxml "$NET" | sed -n '/<dns>/,/<\/dns>/p' || true
echo ">>NET_INFO"
`, netName, gw, orHubIP(m), orClusterName(m), orBaseDomain(m)) + ensureGuestsScript(m, guests) + "\necho VINFRA_OK=1\n"

	out, _, err := e.inv.RunScript(j.InventoryID, script)
	if err != nil {
		return fmt.Errorf("vInfra: %v (%s)", err, truncate(out, 800))
	}
	if !strings.Contains(out, "VINFRA_OK=1") || !strings.Contains(out, "GUESTS_DEFINED=1") {
		return fmt.Errorf("vInfra incomplete: %s", truncate(out, 800))
	}
	e.log(j, StageVInfra, "check guests: virsh --connect qemu:///system list --all")
	e.log(j, StageVInfra, "host stdout:\n%s", trimHostOut(out))
	e.setStage(j, StageVInfra, StageOK,
		fmt.Sprintf("%d guest domain(s) defined on qemu:///system (shut off until ISO boot)", len(guests)),
		out)
	return nil
}

func (e *Engine) stageOCP(j *Job) error {
	m, err := e.mockups.Get(j.MockUpID)
	if err != nil {
		return err
	}
	if isGapPlaceholder(m.Spec.Gaps.PullSecretFile) || isGapPlaceholder(m.Spec.Gaps.SSHPublicKeyFile) {
		return blocked("OCP-MGMT needs real pull-secret and SSH public key paths before agent ISO create (placeholders still set)")
	}

	remoteRoot := remoteWorkRoot(m.Metadata.Name)
	img := EEImage()
	hubName := m.Spec.Hub.Hostname
	if hubName == "" {
		hubName = "hub-sno"
	}
	hubMAC := m.Spec.Hub.MAC
	if hubMAC == "" {
		hubMAC = "52:54:00:13:00:20"
	}
	hubIP := m.Spec.Hub.IP
	if hubIP == "" {
		hubIP = "10.77.30.20"
	}
	baseDomain := m.Spec.BaseDomain
	if baseDomain == "" {
		baseDomain = "lab.example.net"
	}
	machineCIDR := m.Spec.Network.MachineCIDR
	if machineCIDR == "" {
		machineCIDR = "10.77.30.0/24"
	}
	clusterName := m.Metadata.Name
	if clusterName == "" {
		clusterName = "hub"
	}

	pullBytes, err := os.ReadFile(m.Spec.Gaps.PullSecretFile)
	if err != nil {
		return fmt.Errorf("read pull secret: %w", err)
	}
	sshBytes, err := os.ReadFile(m.Spec.Gaps.SSHPublicKeyFile)
	if err != nil {
		return fmt.Errorf("read ssh public key: %w", err)
	}
	pullRemote := remoteRoot + "/hub/pull-secret.json"
	sshRemote := remoteRoot + "/hub/ssh.pub"
	e.log(j, StageOCP, "stage secrets → %s , %s", pullRemote, sshRemote)
	_ = e.SaveJob(j)
	if err := e.inv.WriteRemoteFile(j.InventoryID, pullRemote, pullBytes); err != nil {
		return err
	}
	if err := e.inv.WriteRemoteFile(j.InventoryID, sshRemote, sshBytes); err != nil {
		return err
	}
	// Private key (if present next to .pub) lets wait loop SSH to hub for disk/boot checks.
	if priv := strings.TrimSuffix(m.Spec.Gaps.SSHPublicKeyFile, ".pub"); priv != m.Spec.Gaps.SSHPublicKeyFile {
		if b, err := os.ReadFile(priv); err == nil && len(b) > 0 {
			_ = e.inv.WriteRemoteFile(j.InventoryID, remoteRoot+"/hub/id_install", b)
			_, _, _ = e.inv.RunScript(j.InventoryID, fmt.Sprintf("chmod 600 %q || true", remoteRoot+"/hub/id_install"))
		}
	}

	// Minimal install-config + agent-config for agent-based SNO.
	ic := fmt.Sprintf(`apiVersion: v1
baseDomain: %s
metadata:
  name: %s
controlPlane:
  name: master
  replicas: 1
  architecture: amd64
  hyperthreading: Enabled
compute:
- name: worker
  replicas: 0
  architecture: amd64
networking:
  networkType: OVNKubernetes
  machineNetwork:
  - cidr: %s
  clusterNetwork:
  - cidr: 10.128.0.0/14
    hostPrefix: 23
  serviceNetwork:
  - 172.30.0.0/16
platform:
  none: {}
pullSecret: '%s'
sshKey: |
  %s
`, baseDomain, clusterName, machineCIDR,
		strings.ReplaceAll(strings.TrimSpace(string(pullBytes)), "'", "''"),
		strings.TrimSpace(string(sshBytes)))

	ac := fmt.Sprintf(`apiVersion: v1alpha1
kind: AgentConfig
metadata:
  name: %s
rendezvousIP: %s
hosts:
- hostname: %s
  role: master
  interfaces:
  - name: enp1s0
    macAddress: "%s"
  networkConfig:
    interfaces:
    - name: enp1s0
      type: ethernet
      state: up
      mac-address: "%s"
      ipv4:
        enabled: true
        dhcp: false
        address:
        - ip: %s
          prefix-length: 24
    dns-resolver:
      config:
        server:
        - %s
    routes:
      config:
      - destination: 0.0.0.0/0
        next-hop-address: %s
        next-hop-interface: enp1s0
        table-id: 254
`, clusterName, hubIP, hubName, hubMAC, hubMAC, hubIP, orDNS(m), orGateway(m))

	if err := e.inv.WriteRemoteFile(j.InventoryID, remoteRoot+"/hub/install-config.yaml", []byte(ic)); err != nil {
		return err
	}
	if err := e.inv.WriteRemoteFile(j.InventoryID, remoteRoot+"/hub/agent-config.yaml", []byte(ac)); err != nil {
		return err
	}
	e.log(j, StageOCP, "wrote install-config + agent-config; run openshift-install agent create image via EE")
	_ = e.SaveJob(j)

	script := fmt.Sprintf(`set -eu
ROOT=%q
EE_IMAGE=%q
HUB=%q
export LIBVIRT_DEFAULT_URI="${LIBVIRT_DEFAULT_URI:-qemu:///system}"
mkdir -p "$ROOT/hub"
# Keep copies install-config needs (installer consumes/moves install-config.yaml)
cp -f "$ROOT/hub/install-config.yaml" "$ROOT/hub/install-config.yaml.bak" 2>/dev/null || true
echo "AGENT_CREATE_START"
set +e
podman run --rm \
  -v "$ROOT/hub:/output:Z" \
  --entrypoint /usr/local/bin/openshift-install \
  "$EE_IMAGE" agent create image --dir /output
RC=$?
set -e
if [ "$RC" -ne 0 ]; then
  echo "AGENT_CREATE_FAILED=$RC"
  virsh --connect qemu:///system list --all || true
  echo OCP_SOFT_FAIL=1
  exit 0
fi
ISO=$(ls -1 "$ROOT/hub"/agent*.iso 2>/dev/null | head -1 || true)
echo "ISO=$ISO"
POOL=%q
POOL_PATH=$(virsh pool-dumpxml "$POOL" 2>/dev/null | sed -n 's/.*<path>\([^<]*\)<\/path>.*/\1/p' | head -1)
[ -n "$POOL_PATH" ] || POOL_PATH=%q
mkdir -p "$POOL_PATH"
chmod o+x /vm-disks "$(dirname "$POOL_PATH")" "$POOL_PATH" 2>/dev/null || true
chmod 755 "$POOL_PATH" 2>/dev/null || true
if [ -n "$ISO" ] && virsh dominfo "$HUB" >/dev/null 2>&1; then
  # Copy out of the podman-written hub dir into the libvirt pool on /vm-disks.
  ISO_POOL="$POOL_PATH/${HUB}-agent.iso"
  cp -f "$ISO" "$ISO_POOL"
  chmod a+r "$ISO_POOL"
  if command -v chcon >/dev/null 2>&1; then
    chcon -t virt_content_t -l s0 "$ISO_POOL" 2>/dev/null || true
  fi
  echo "ISO_POOL=$ISO_POOL"
  virsh destroy "$HUB" 2>/dev/null || true
  virsh change-media "$HUB" sda --eject --config 2>/dev/null || true
  virsh change-media "$HUB" sda "$ISO_POOL" --insert --config 2>/dev/null \
    || virsh change-media "$HUB" hdc "$ISO_POOL" --insert --config 2>/dev/null \
    || virsh attach-disk "$HUB" "$ISO_POOL" sda --type cdrom --mode readonly --persistent 2>/dev/null \
    || true
  # Empty qcow boots hd first and powers off; agent ISO must be first.
  if command -v virt-xml >/dev/null 2>&1; then
    virt-xml "$HUB" --edit --boot cdrom,hd --define 2>/dev/null \
      || virt-xml "$HUB" --edit --boot order=cdrom,hd --define 2>/dev/null || true
    echo "BOOT_ORDER=cdrom,hd"
  else
    echo "BOOT_ORDER_WARN=no-virt-xml"
  fi
  if virsh start "$HUB"; then
    echo "HUB_START_OK=1"
  else
    echo "HUB_START_FAILED=1"
    virsh start "$HUB" 2>&1 || true
  fi
  sleep 8
  STATE=$(virsh --connect qemu:///system domstate "$HUB" 2>/dev/null | tr -d '\n' || true)
  echo "HUB_STATE=$STATE"
  sleep 12
  STATE2=$(virsh --connect qemu:///system domstate "$HUB" 2>/dev/null | tr -d '\n' || true)
  echo "HUB_STATE_AFTER=$STATE2"
  if [ "$STATE2" = "running" ]; then
    echo "HUB_RUNNING=1"
  fi
  echo "CONSOLE_HINT=virsh --connect qemu:///system console $HUB"
fi
echo "VIRSH_SYSTEM<<"
virsh --connect qemu:///system list --all || true
echo ">>VIRSH_SYSTEM"
echo OCP_PREP_OK=1
`, remoteRoot, img, hubName, HostPoolName, HostImagesDir)

	out, _, err := e.inv.RunScript(j.InventoryID, script)
	if err != nil {
		return fmt.Errorf("OCP: %v (%s)", err, truncate(out, 800))
	}
	e.log(j, StageOCP, "check: virsh --connect qemu:///system list --all")
	e.log(j, StageOCP, "host stdout:\n%s", trimHostOut(out))
	_ = e.SaveJob(j)
	if strings.Contains(out, "OCP_SOFT_FAIL=1") {
		reason := "openshift-install agent create image failed"
		low := strings.ToLower(out)
		switch {
		case strings.Contains(low, "illegal base64") || strings.Contains(low, "unable to load --registry-config"):
			reason = "pull-secret is not a valid registry auth (DEV stub or corrupt file) — set a real pull-secret in Wizard or MOCK_ME_DEV_PULL_SECRET, then Clean + Deploy"
		case strings.Contains(low, "unknown field") || strings.Contains(low, "not valid networkstate"):
			reason = "agent-config nmstate invalid (check static network YAML)"
		case strings.Contains(low, `exec: "nmstatectl"`) || strings.Contains(low, "nmstatectl: executable file not found"):
			reason = "EE image missing nmstatectl (rebuild/push mock-me-ee with nmstate)"
		case strings.Contains(low, "unauthorized") || strings.Contains(low, "authentication required") ||
			strings.Contains(low, "pull secret") && (strings.Contains(low, "denied") || strings.Contains(low, "401") || strings.Contains(low, "403")):
			reason = "pull-secret unauthorized for quay.io/openshift-release-dev — use a real OpenShift pull secret"
		case strings.Contains(low, "cannot generate iso") || strings.Contains(low, "configuration errors"):
			reason = "agent create image failed on config (nmstate/install-config) — check host stdout above"
		case strings.Contains(low, "pull secret") || strings.Contains(low, "pull-secret"):
			reason = "pull-secret invalid or unauthorized for release image"
		}
		return blocked(fmt.Sprintf(
			"OCP-MGMT blocked — guest domains exist (shut off) but agent ISO was not created (%s). Hub %s not started. Fix EE/pull-secret/agent-config, then Clean + Deploy. Check: virsh --connect qemu:///system list --all",
			reason, hubName))
	}
	if !strings.Contains(out, "OCP_PREP_OK=1") {
		return fmt.Errorf("OCP incomplete: %s", truncate(out, 800))
	}
	if !strings.Contains(out, "HUB_RUNNING=1") {
		extra := ""
		if strings.Contains(out, "HUB_START_FAILED=1") {
			extra = " (virsh start failed — check qemu can read pool disks/ISO under /vm-disks/mock-me)"
		} else if strings.Contains(out, "HUB_START_OK=1") {
			extra = " (started then shut off — boot order must be cdrom,hd with agent ISO inserted)"
		}
		return blocked(fmt.Sprintf(
			"Agent ISO may exist but hub domain %s is not staying running%s. Guests: virsh --connect qemu:///system list --all ; console: virsh --connect qemu:///system console %s",
			hubName, extra, hubName))
	}

	e.log(j, StageOCP, "hub %s running with agent ISO — waiting for install-complete", hubName)
	_ = e.SaveJob(j)
	kcBody, err := e.waitForHubInstall(j, remoteRoot, hubName, hubIP,
		fmt.Sprintf("api.%s.%s", clusterName, baseDomain),
		fmt.Sprintf("api-int.%s.%s", clusterName, baseDomain))
	if err != nil {
		return err
	}
	localKC, err := e.persistHubKubeconfig(j, m.Metadata.Name, kcBody)
	if err != nil {
		return fmt.Errorf("persist kubeconfig: %w", err)
	}
	e.setStage(j, StageOCP, StageOK,
		fmt.Sprintf("OCP-MGMT install complete — hub %s API up; kubeconfig %s", hubName, localKC),
		out)
	return nil
}

func orGateway(m *mockup.MockUp) string {
	if m.Spec.Network.Gateway != "" {
		return m.Spec.Network.Gateway
	}
	return "10.77.30.1"
}

func orDNS(m *mockup.MockUp) string {
	if m.Spec.Network.DNS != "" {
		return m.Spec.Network.DNS
	}
	return orGateway(m)
}

func (e *Engine) stageACM(j *Job) error {
	m, err := e.mockups.Get(j.MockUpID)
	if err != nil {
		return err
	}
	if !m.Spec.Hub.InstallACM && !m.Spec.ACM.Enabled {
		e.setStage(j, StageACM, StageOK, "ACM not enabled on this MockUp style/path — skipped", "")
		return nil
	}
	kc := m.Spec.Gaps.HubKubeconfig
	if isGapPlaceholder(kc) || kc == "" {
		return fmt.Errorf("ACM failed — need a live hub kubeconfig after OCP-MGMT finishes (gap empty)")
	}
	if !hubKubeconfigReady(kc) {
		return fmt.Errorf("ACM failed — hub kubeconfig not ready at %s", kc)
	}

	remoteRoot := remoteWorkRoot(m.Metadata.Name)
	mceCh := m.Spec.ACM.MCEChannel
	acmCh := m.Spec.ACM.ACMChannel
	e.log(j, StageACM, "install MCE+ACM via EE oc → %s/acm (acm=%s mce=%s) kubeconfig=%s",
		remoteRoot, acmCh, mceCh, kc)
	_ = e.SaveJob(j)

	if err := e.installACMOperators(j, remoteRoot, kc, mceCh, acmCh); err != nil {
		return err
	}
	e.setStage(j, StageACM, StageOK,
		fmt.Sprintf("MCE + ACM CSVs Available (channels mce=%s acm=%s)", mceCh, acmCh),
		"")
	return nil
}

func (e *Engine) stageSpokes(j *Job) error {
	m, err := e.mockups.Get(j.MockUpID)
	if err != nil {
		return err
	}
	if m.Spec.Style == mockup.StyleSingleSNOOCP || len(m.Spec.Clusters) == 0 {
		e.setStage(j, StageSpokes, StageOK, "No OCP-DEPLOY spokes on this MockUp — done", "")
		return nil
	}

	guests := buildGuests(m)
	var spokeNames []string
	var namesShell strings.Builder
	for _, g := range guests {
		if g.Role == "spoke" {
			spokeNames = append(spokeNames, g.Name)
			fmt.Fprintf(&namesShell, " %s", shellQuote(g.Name))
		}
	}
	missingISO := 0
	for _, c := range m.Spec.Clusters {
		e.log(j, StageSpokes, "cluster %s discoveryISO=%s", c.Name, orDash(c.DiscoveryISO))
		if isGapPlaceholder(c.DiscoveryISO) {
			missingISO++
		}
	}
	e.log(j, StageSpokes, "expect spoke domains: %s", strings.Join(spokeNames, ", "))
	_ = e.SaveJob(j)

	script := fmt.Sprintf(`set -eu
export LIBVIRT_DEFAULT_URI="${LIBVIRT_DEFAULT_URI:-qemu:///system}"
echo "SPOKE_CHECK<<"
virsh --connect qemu:///system list --all || true
echo ">>SPOKE_CHECK"
MISSING=0
for n in%s; do
  if virsh dominfo "$n" >/dev/null 2>&1; then
    echo "OK $n ($(virsh domstate "$n" 2>/dev/null | tr -d '\n'))"
  else
    echo "MISSING $n"
    MISSING=$((MISSING+1))
  fi
done
echo "SPOKE_MISSING=$MISSING"
`, namesShell.String())

	out, _, err := e.inv.RunScript(j.InventoryID, script)
	if err != nil {
		return fmt.Errorf("spokes: %v (%s)", err, truncate(out, 400))
	}
	e.log(j, StageSpokes, "host stdout:\n%s", trimHostOut(out))
	if strings.Contains(out, "SPOKE_MISSING=") && !strings.Contains(out, "SPOKE_MISSING=0") {
		return fmt.Errorf("spoke domains missing after vInfra — check virt-install / pool perms: %s", truncate(out, 600))
	}
	if missingISO > 0 {
		return blocked(fmt.Sprintf(
			"OCP-DEPLOY blocked (out of scope for lab-e2e) — %d spoke domain(s) defined (shut off) but %d cluster(s) still need discovery ISO paths (ACM InfraEnv). Not auto-booted. Check: virsh --connect qemu:///system list --all",
			len(spokeNames), missingISO))
	}
	return blocked(fmt.Sprintf(
		"OCP-DEPLOY blocked (out of scope for lab-e2e) — %d spoke domain(s) defined with discovery ISO paths set, but attach/boot + ACM spoke bring-up is not automated yet.",
		len(spokeNames)))
}

func hubKubeconfigReady(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	st, err := os.Stat(path)
	if err != nil || st.IsDir() || st.Size() < 32 {
		return false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	s := string(b)
	return strings.Contains(s, "clusters:") || strings.Contains(s, "apiVersion:")
}

func orDash(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "(missing)"
	}
	return s
}

func orHubIP(m *mockup.MockUp) string {
	if m != nil && strings.TrimSpace(m.Spec.Hub.IP) != "" {
		return strings.TrimSpace(m.Spec.Hub.IP)
	}
	return "10.77.30.20"
}

func orClusterName(m *mockup.MockUp) string {
	if m != nil && strings.TrimSpace(m.Metadata.Name) != "" {
		return strings.TrimSpace(m.Metadata.Name)
	}
	return "hub"
}

func orBaseDomain(m *mockup.MockUp) string {
	if m != nil && strings.TrimSpace(m.Spec.BaseDomain) != "" {
		return strings.TrimSpace(m.Spec.BaseDomain)
	}
	return "lab.example.net"
}

func safeUser(j *Job) string {
	// Endpoint is user@host — prefer dasm from inventory via host name fallback
	if ep := j.HostEndpoint; strings.Contains(ep, "@") {
		return strings.SplitN(ep, "@", 2)[0]
	}
	return "dasm"
}

func isGapPlaceholder(p string) bool {
	p = strings.TrimSpace(p)
	return p == "" || strings.HasPrefix(p, "$")
}

func trimHostOut(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 1200 {
		return s[:1200] + "…"
	}
	return s
}
