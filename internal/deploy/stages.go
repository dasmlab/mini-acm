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
	e.log(j, StageEE, "check podman + openshift-install + quay.io/ansible/creator-ee on %s", j.HostEndpoint)
	_ = e.SaveJob(j)
	script := `set -eu
echo "EE_CHECK_START"
if command -v podman >/dev/null 2>&1; then
  echo "podman=$(podman --version 2>/dev/null | head -1)"
  echo "HAS_PODMAN=1"
else
  echo "HAS_PODMAN=0"
fi
if command -v openshift-install >/dev/null 2>&1; then
  echo "openshift-install=$(openshift-install version 2>&1 | sed -n '1p')"
  echo "HAS_INSTALLER=1"
else
  echo "HAS_INSTALLER=0"
fi
if [ "${HAS_PODMAN:-0}" != "1" ]; then
  echo "EE_FAIL=podman"
  exit 0
fi
if [ "${HAS_INSTALLER:-0}" != "1" ]; then
  echo "EE_FAIL=openshift-install"
  exit 0
fi
# Lightweight runner image check (pull only if missing — keep lab friendly)
if ! podman image exists quay.io/ansible/creator-ee:latest 2>/dev/null; then
  echo "PULLING creator-ee (may take a minute)…"
  podman pull quay.io/ansible/creator-ee:latest
fi
podman image exists quay.io/ansible/creator-ee:latest
# Smoke: run a no-op container (no pipefail+head — SIGPIPE 141)
podman run --rm quay.io/ansible/creator-ee:latest ansible --version 2>&1 | sed -n '1,5p' || true
echo "EE_OK=1"
`
	out, _, err := e.inv.RunScript(j.InventoryID, script)
	if err != nil {
		return fmt.Errorf("execution env: %v (%s)", err, truncate(out, 500))
	}
	e.log(j, StageEE, "host stdout:\n%s", trimHostOut(out))
	_ = e.SaveJob(j)
	if strings.Contains(out, "EE_FAIL=podman") {
		return blocked("podman not on inventory host — required for EE/runner; Probe → Fix this (install-podman), then Clean + Deploy")
	}
	if strings.Contains(out, "EE_FAIL=openshift-install") {
		return blocked("openshift-install not on inventory host — required before OCP-MGMT; install the client on the MACHINE-HOST (Inventory Probe will flag it), then Clean + Deploy. Plan files are not staged until EE passes.")
	}
	if !strings.Contains(out, "EE_OK=1") {
		return fmt.Errorf("EE check incomplete: %s", truncate(out, 500))
	}
	e.setStage(j, StageEE, StageOK, "Podman + openshift-install + Ansible creator-EE ready on inventory host", out)
	e.log(j, StageEE, "host stdout:\n%s", trimHostOut(out))
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
	e.log(j, StageVInfra, "libvirt pool=%q net=%q cidr=%s gw=%s on %s", pool, netName, cidr, gw, j.HostEndpoint)
	_ = e.SaveJob(j)
	script := fmt.Sprintf(`set -eu
echo VINFRA_START
export LIBVIRT_DEFAULT_URI="${LIBVIRT_DEFAULT_URI:-qemu:///system}"
systemctl is-active libvirtd 2>/dev/null || sudo -n systemctl start libvirtd
systemctl is-active libvirtd
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
echo VINFRA_OK=1
`, pool, netName, gw)

	out, _, err := e.inv.RunScript(j.InventoryID, script)
	if err != nil {
		return fmt.Errorf("vInfra: %v (%s)", err, truncate(out, 600))
	}
	if !strings.Contains(out, "VINFRA_OK=1") {
		return fmt.Errorf("vInfra incomplete: %s", truncate(out, 600))
	}
	e.setStage(j, StageVInfra, StageOK,
		fmt.Sprintf("libvirt pool %q + network %q ready (CIDR plan %s)", pool, netName, cidr),
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
		return blocked("OCP-MGMT needs real pull-secret and SSH public key paths in Wizard gaps before ISO/agent install can run (placeholders still set)")
	}

	// Verify remote workspace + prepare hub work dir; kick agent create if openshift-install present.
	remoteRoot := fmt.Sprintf("/home/%s/mock-me-work/%s", safeUser(j), m.Metadata.Name)
	e.log(j, StageOCP, "gaps pullSecret=%s sshPub=%s", m.Spec.Gaps.PullSecretFile, m.Spec.Gaps.SSHPublicKeyFile)
	e.log(j, StageOCP, "prep hub workdir %s/hub (expect %s/out/hub.yaml)", remoteRoot, remoteRoot)
	_ = e.SaveJob(j)
	script := fmt.Sprintf(`set -eu
ROOT=%q
mkdir -p "$ROOT/hub"
test -f "$ROOT/out/hub.yaml"
if command -v openshift-install >/dev/null 2>&1; then
  echo "openshift-install=$(openshift-install version 2>&1 | sed -n '1p')"
  echo "HAS_INSTALLER=1"
else
  echo "HAS_INSTALLER=0"
fi
# VyOS / appliance note
if [ -n "%s" ]; then echo "VYOS_ISO_SET=1"; else echo "VYOS_ISO_SET=0"; fi
virsh list --all || true
echo OCP_PREP_OK=1
`, remoteRoot, m.Spec.Gateway.ISOPath)

	out, _, err := e.inv.RunScript(j.InventoryID, script)
	if err != nil {
		return fmt.Errorf("OCP prep: %v (%s)", err, truncate(out, 500))
	}
	if strings.Contains(out, "HAS_INSTALLER=0") {
		return blocked("openshift-install not on inventory host yet — install the client or run hub create from a provisioner with the binary; plan YAML is staged at " + remoteRoot)
	}
	e.setStage(j, StageOCP, StageOK,
		"OCP prep OK — installer present; hub create can proceed (run agent image next)",
		out)
	return nil
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

	missingISO := 0
	remoteRoot := fmt.Sprintf("/home/%s/mock-me-work/%s", safeUser(j), m.Metadata.Name)
	for _, c := range m.Spec.Clusters {
		iso := c.DiscoveryISO
		if isGapPlaceholder(iso) {
			missingISO++
			e.log(j, StageSpokes, "spoke %s (%s) discoveryISO=MISSING", c.Label, c.Name)
		} else {
			e.log(j, StageSpokes, "spoke %s (%s) discoveryISO=%s", c.Label, c.Name, iso)
		}
	}
	e.log(j, StageSpokes, "list %s/out/cluster-*.yaml → %s/spokes", remoteRoot, remoteRoot)
	_ = e.SaveJob(j)
	script := fmt.Sprintf(`set -eu
ROOT=%q
mkdir -p "$ROOT/spokes"
ls -la "$ROOT/out"/cluster-*.yaml 2>/dev/null || true
COUNT=%d
echo "SPOKE_PLANS=$COUNT"
echo SPOKES_STAGED=1
`, remoteRoot, len(m.Spec.Clusters))

	out, _, err := e.inv.RunScript(j.InventoryID, script)
	if err != nil {
		return fmt.Errorf("spokes: %v (%s)", err, truncate(out, 400))
	}
	if missingISO > 0 {
		return blocked(fmt.Sprintf(
			"%d deployment cluster(s) still need discovery ISO paths (InfraEnv → attach-iso). Cluster YAML is staged on host at %s/out — re-Deploy after ISOs exist.",
			missingISO, remoteRoot))
	}
	e.setStage(j, StageSpokes, StageOK,
		fmt.Sprintf("%d spoke plan(s) staged with discovery ISO paths set", len(m.Spec.Clusters)),
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
