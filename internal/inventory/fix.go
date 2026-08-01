package inventory

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// Fix action ids returned by probe and accepted by Fix.
const (
	FixInstallLibvirt = "install-libvirt"
	FixStartLibvirtd  = "start-libvirtd"
	FixInstallPodman  = "install-podman"
)

// FixReq remediates probe issues on the target over SSH.
// SudoPassword is never persisted; used only for this request (assume registered host + capable user).
type FixReq struct {
	Actions      []string `json:"actions"`                // empty = all fixable from last probe / defaults
	SudoPassword string   `json:"sudoPassword,omitempty"` // optional; blank tries passwordless sudo -n
}

// FixResult is the outcome of a remediating SSH session.
type FixResult struct {
	OK        bool              `json:"ok"`
	Message   string            `json:"message"`
	Log       []string          `json:"log,omitempty"`
	Actions   []string          `json:"actions"`
	Probe     *ProbeResult      `json:"probe,omitempty"` // re-probe after fix
	Host      *MachineHost      `json:"host,omitempty"`
	Facts     map[string]string `json:"facts,omitempty"`
	CheckedAt string            `json:"checkedAt"`
}

// Fix installs/starts missing pieces on the MACHINE-HOST (libvirt, podman, …).
// Direction: package bootstrap now; later the same path pulls a slim Ansible EE / runner
// agent image (podman) with injected SSH key for probe/orchestrate from cluster or host.
func (s *Store) Fix(id string, req FixReq) (*FixResult, error) {
	h, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	out := &FixResult{
		Actions:   req.Actions,
		Facts:     map[string]string{},
		CheckedAt: now,
	}

	actions := req.Actions
	if len(actions) == 0 {
		actions = fixableActionsFromHost(h)
	}
	if len(actions) == 0 {
		out.Message = "nothing to fix — re-probe or host already ready"
		out.OK = true
		return out, nil
	}
	out.Actions = actions

	client, err := dialSSH(h)
	if err != nil {
		out.Message = err.Error()
		return out, nil
	}
	defer client.Close()

	for _, a := range actions {
		lines, err := runFixAction(client, a, req.SudoPassword)
		out.Log = append(out.Log, lines...)
		if err != nil {
			out.Message = fmt.Sprintf("fix %s failed: %v", a, err)
			out.Log = append(out.Log, out.Message)
			// still re-probe to refresh status
			break
		}
		out.Log = append(out.Log, fmt.Sprintf("✓ %s", a))
	}

	// Always re-probe so UI gets red/yellow/green + updated issues
	probe := probeHost(h)
	applyProbeToHost(h, probe)
	_ = s.Save(h)
	out.Probe = probe
	out.Host = h
	out.OK = probe.LibvirtReady || (out.Message == "" && probe.AuthOK)
	if out.Message == "" {
		if probe.LibvirtReady {
			out.Message = "fix applied — host ready (libvirt up)"
		} else if probe.AuthOK {
			out.Message = "fix ran — still partial; check log / sudo / subscription"
			out.OK = false
		} else {
			out.Message = "fix ran but probe no longer authenticates"
			out.OK = false
		}
	}
	return out, nil
}

func fixableActionsFromHost(h *MachineHost) []string {
	seen := map[string]bool{}
	var out []string
	for _, iss := range h.Issues {
		if !iss.Fixable || iss.FixAction == "" || seen[iss.FixAction] {
			continue
		}
		seen[iss.FixAction] = true
		out = append(out, iss.FixAction)
	}
	return out
}

func runFixAction(client *ssh.Client, action, sudoPassword string) ([]string, error) {
	switch action {
	case FixInstallLibvirt:
		return sudoScript(client, sudoPassword, `
set -e
# Lab hosts sometimes hit RH GPG key lag after fresh registration — try normal, then nogpgcheck.
if ! dnf install -y libvirt libvirt-client libvirt-daemon-kvm qemu-kvm virt-install; then
  echo "dnf GPG/normal install failed — retrying with --nogpgcheck (lab)"
  dnf install -y --nogpgcheck libvirt libvirt-client libvirt-daemon-kvm qemu-kvm virt-install
fi
systemctl enable --now libvirtd 2>/dev/null || systemctl enable --now virtqemud 2>/dev/null || systemctl enable --now libvirt
# common lab: allow user libvirt group if present
if getent group libvirt >/dev/null 2>&1; then
  usermod -aG libvirt "$(logname 2>/dev/null || echo "${SUDO_USER:-$USER}")" 2>/dev/null || true
fi
# firewall: libvirt API + guest display ports (lab)
if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
  firewall-cmd --permanent --add-service=libvirt || true
  firewall-cmd --permanent --add-port=16509/tcp || true
  firewall-cmd --permanent --add-port=5900-5920/tcp || true
  firewall-cmd --permanent --zone=trusted --add-interface=virbr0 || true
  firewall-cmd --reload || true
fi
systemctl is-active libvirtd || systemctl is-active virtqemud || systemctl is-active libvirt
command -v virsh
virsh version | head -5
`, "install-libvirt")
	case FixStartLibvirtd:
		return sudoScript(client, sudoPassword, `
set -e
# If unit missing, packages were never installed — install then start.
if ! systemctl list-unit-files libvirtd.service 2>/dev/null | grep -q libvirtd.service \
   && ! systemctl list-unit-files virtqemud.service 2>/dev/null | grep -q virtqemud.service; then
  echo "libvirtd unit missing — installing libvirt stack first"
  if ! dnf install -y libvirt libvirt-client libvirt-daemon-kvm qemu-kvm virt-install; then
    echo "dnf GPG/normal install failed — retrying with --nogpgcheck (lab)"
    dnf install -y --nogpgcheck libvirt libvirt-client libvirt-daemon-kvm qemu-kvm virt-install
  fi
fi
systemctl enable --now libvirtd 2>/dev/null || systemctl enable --now virtqemud 2>/dev/null || systemctl enable --now libvirt
systemctl is-active libvirtd || systemctl is-active virtqemud || systemctl is-active libvirt
if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
  firewall-cmd --permanent --add-service=libvirt || true
  firewall-cmd --permanent --add-port=16509/tcp || true
  firewall-cmd --reload || true
fi
`, "start-libvirtd")
	case FixInstallPodman:
		return sudoScript(client, sudoPassword, `
set -e
if ! dnf install -y podman; then
  echo "dnf GPG/normal install failed — retrying with --nogpgcheck (lab)"
  dnf install -y --nogpgcheck podman
fi
command -v podman
podman --version
# EE / runner agent image pull is deferred (MOCK_ME_AGENT_IMAGE later)
`, "install-podman")
	default:
		return nil, fmt.Errorf("unknown fix action %q", action)
	}
}

func sudoScript(client *ssh.Client, sudoPassword, script, label string) ([]string, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	var buf bytes.Buffer
	session.Stdout = &buf
	session.Stderr = &buf

	// Feed sudo password on stdin when provided; otherwise require passwordless sudo.
	// Use explicit bash -lc so failures from systemctl/dnf always surface on stderr.
	inner := "bash -s"
	var cmd string
	if strings.TrimSpace(sudoPassword) != "" {
		session.Stdin = strings.NewReader(sudoPassword + "\n" + script)
		cmd = "sudo -S -p '' " + inner
	} else {
		session.Stdin = strings.NewReader(script)
		cmd = "sudo -n " + inner
	}

	err = session.Run(cmd)
	out := strings.TrimSpace(buf.String())
	lines := []string{fmt.Sprintf("--- %s ---", label)}
	if out != "" {
		for _, ln := range strings.Split(out, "\n") {
			lines = append(lines, ln)
		}
	}
	if err != nil {
		hint := ""
		if strings.TrimSpace(sudoPassword) == "" {
			hint = " (try providing sudo password in Fix dialog, or configure passwordless sudo)"
		}
		if out == "" {
			hint += " (no remote output — unit may be missing; try Fix install-libvirt)"
		}
		return lines, fmt.Errorf("%v%s — output: %s", err, hint, truncate(out, 800))
	}
	return lines, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func dialSSH(h *MachineHost) (*ssh.Client, error) {
	addr := net.JoinHostPort(h.EffectiveSSHHost(), strconv.Itoa(h.SSHPort))
	keyPath, key, err := loadIdentity(h.IdentityFile)
	if err != nil {
		return nil, fmt.Errorf("SSH identity: %v", err)
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("invalid key %s: %v", keyPath, err)
	}
	cfg := &ssh.ClientConfig{
		User:            h.SSHUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         20 * time.Second,
	}
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("SSH dial %s: %v", h.Endpoint(), err)
	}
	return client, nil
}
