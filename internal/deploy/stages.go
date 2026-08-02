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

	remoteRoot := fmt.Sprintf("/home/%s/mock-me-work/%s", safeUser(j), m.Metadata.Name)
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
	if pool == "" {
		pool = "default"
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
	e.log(j, StageVInfra, "libvirt pool=%q net=%q cidr=%s — materialize %d guest(s)", pool, netName, cidr, len(guests))
	for _, g := range guests {
		e.log(j, StageVInfra, "plan guest %s role=%s cpu=%d memMiB=%d diskGiB=%d mac=%s",
			g.Name, g.Role, g.CPU, g.MemoryMiB, g.DiskGiB, g.MAC)
	}
	_ = e.SaveJob(j)

	script := fmt.Sprintf(`set -eu
echo VINFRA_START
export LIBVIRT_DEFAULT_URI="${LIBVIRT_DEFAULT_URI:-qemu:///system}"
systemctl is-active libvirtd 2>/dev/null || sudo -n systemctl start libvirtd
systemctl is-active libvirtd
command -v virt-install >/dev/null || { echo "virt-install missing — Fix this install-libvirt"; exit 1; }
POOL=%q
NET=%q
GW=%q
if ! virsh pool-info "$POOL" >/dev/null 2>&1; then
  mkdir -p "$HOME/libvirt-images"
  virsh pool-define-as "$POOL" dir --target "$HOME/libvirt-images" || true
  virsh pool-build "$POOL" || true
  virsh pool-start "$POOL" || true
  virsh pool-autostart "$POOL" || true
fi
echo "POOL_INFO<<"
virsh pool-info "$POOL" || true
echo ">>POOL_INFO"
if ! virsh net-info "$NET" >/dev/null 2>&1; then
  cat > /tmp/mock-me-net.xml <<EOF
<network>
  <name>$NET</name>
  <forward mode='nat'/>
  <bridge name='virbr-mm' stp='on' delay='0'/>
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
echo "NET_INFO<<"
virsh net-info "$NET" || true
echo ">>NET_INFO"
`, pool, netName, gw) + ensureGuestsScript(m, guests) + "\necho VINFRA_OK=1\n"

	out, _, err := e.inv.RunScript(j.InventoryID, script)
	if err != nil {
		return fmt.Errorf("vInfra: %v (%s)", err, truncate(out, 800))
	}
	if !strings.Contains(out, "VINFRA_OK=1") || !strings.Contains(out, "GUESTS_DEFINED=1") {
		return fmt.Errorf("vInfra incomplete: %s", truncate(out, 800))
	}
	e.setStage(j, StageVInfra, StageOK,
		fmt.Sprintf("libvirt pool %q + net %q + %d guest domain(s) defined (shut off until ISO boot)", pool, netName, len(guests)),
		out)
	e.log(j, StageVInfra, "host stdout:\n%s", trimHostOut(out))
	_ = e.SaveJob(j)
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

	remoteRoot := fmt.Sprintf("/home/%s/mock-me-work/%s", safeUser(j), m.Metadata.Name)
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
        gateway: %s
`, clusterName, hubIP, hubName, hubMAC, hubMAC, hubIP, orGateway(m))

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
  # Guests already exist from vInfra — do not hard-fail the whole line for a stub pull-secret.
  virsh list --all || true
  echo OCP_SOFT_FAIL=1
  exit 0
fi
ISO=$(ls -1 "$ROOT/hub"/agent*.iso 2>/dev/null | head -1 || true)
echo "ISO=$ISO"
if [ -n "$ISO" ] && virsh dominfo "$HUB" >/dev/null 2>&1; then
  # Attach ISO as CDROM and boot hub SNO
  virsh change-media "$HUB" sda "$ISO" --insert --config 2>/dev/null \
    || virsh change-media "$HUB" hdc "$ISO" --insert --config 2>/dev/null \
    || virsh attach-disk "$HUB" "$ISO" sda --type cdrom --mode readonly --persistent 2>/dev/null \
    || true
  virsh start "$HUB" || true
  echo "HUB_BOOTED=1"
fi
virsh list --all || true
echo OCP_PREP_OK=1
`, remoteRoot, img, hubName)

	out, _, err := e.inv.RunScript(j.InventoryID, script)
	if err != nil {
		return fmt.Errorf("OCP: %v (%s)", err, truncate(out, 800))
	}
	e.log(j, StageOCP, "host stdout:\n%s", trimHostOut(out))
	_ = e.SaveJob(j)
	if strings.Contains(out, "OCP_SOFT_FAIL=1") {
		e.setStage(j, StageOCP, StageOK,
			fmt.Sprintf("Guest domains exist; agent ISO not created yet (openshift-install failed — often stub/invalid pull-secret). Hub %s left shut off. Fix pull-secret and re-Deploy/Clean.", hubName),
			out)
		return nil
	}
	if !strings.Contains(out, "OCP_PREP_OK=1") {
		return fmt.Errorf("OCP incomplete: %s", truncate(out, 800))
	}
	msg := fmt.Sprintf("Agent ISO path ready; hub domain %s", hubName)
	if strings.Contains(out, "HUB_BOOTED=1") {
		msg = fmt.Sprintf("Agent ISO attached — started hub domain %s", hubName)
	}
	e.setStage(j, StageOCP, StageOK, msg, out)
	return nil
}

func orGateway(m *mockup.MockUp) string {
	if m.Spec.Network.Gateway != "" {
		return m.Spec.Network.Gateway
	}
	return "10.77.30.1"
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
		return blocked("ACM install needs a live hub kubeconfig (after OCP-MGMT is up). Set hub kubeconfig path in Wizard, then re-Deploy.")
	}

	// Check if kubeconfig is reachable from API pod OR stage manifests to host for later.
	remoteRoot := fmt.Sprintf("/home/%s/mock-me-work/%s", safeUser(j), m.Metadata.Name)
	e.log(j, StageACM, "stage ACM channels → %s/acm (acm=%s mce=%s) kubeconfig=%s",
		remoteRoot, m.Spec.ACM.ACMChannel, m.Spec.ACM.MCEChannel, kc)
	_ = e.SaveJob(j)
	script := fmt.Sprintf(`set -eu
ROOT=%q
mkdir -p "$ROOT/acm"
echo "ACM channel=%s MCE=%s" > "$ROOT/acm/channels.txt"
if command -v oc >/dev/null 2>&1; then
  echo HAS_OC=1
else
  echo HAS_OC=0
fi
echo ACM_STAGED=1
`, remoteRoot, m.Spec.ACM.ACMChannel, m.Spec.ACM.MCEChannel)

	out, _, err := e.inv.RunScript(j.InventoryID, script)
	if err != nil {
		return fmt.Errorf("ACM stage: %v (%s)", err, truncate(out, 400))
	}
	e.setStage(j, StageACM, StageOK, "ACM channel manifests staged on host — apply after hub kubeconfig is live", out)
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
	e.log(j, StageSpokes, "expect spoke domains: %s", strings.Join(spokeNames, ", "))
	_ = e.SaveJob(j)

	script := fmt.Sprintf(`set -eu
export LIBVIRT_DEFAULT_URI="${LIBVIRT_DEFAULT_URI:-qemu:///system}"
echo "SPOKE_CHECK<<"
virsh list --all || true
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
echo SPOKES_STAGED=1
`, namesShell.String())

	out, _, err := e.inv.RunScript(j.InventoryID, script)
	if err != nil {
		return fmt.Errorf("spokes: %v (%s)", err, truncate(out, 400))
	}
	if strings.Contains(out, "SPOKE_MISSING=") && !strings.Contains(out, "SPOKE_MISSING=0") {
		return fmt.Errorf("spoke domains missing after vInfra — check virt-install / pool perms: %s", truncate(out, 600))
	}
	e.setStage(j, StageSpokes, StageOK,
		fmt.Sprintf("%d spoke domain(s) defined (shut off). Discovery ISO attach is next after ACM InfraEnv — not auto-booted yet.", len(spokeNames)),
		out)
	return nil
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
