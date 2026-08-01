package inventory

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// ProbeIssue is a structured finding from probe (drives Fix this in UI).
type ProbeIssue struct {
	ID        string `json:"id" yaml:"id"`
	Severity  string `json:"severity" yaml:"severity"` // error | warn
	Message   string `json:"message" yaml:"message"`
	Fixable   bool   `json:"fixable" yaml:"fixable"`
	FixAction string `json:"fixAction,omitempty" yaml:"fixAction,omitempty"`
}

// Probe connects over SSH and smoke-checks readiness for orchestration.
// Status: unreachable (red) | partial (yellow) | reachable/ready (green).
func (s *Store) Probe(id string) (*ProbeResult, error) {
	h, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	res := probeHost(h)
	applyProbeToHost(h, res)
	_ = s.Save(h)
	res.Host = h
	return res, nil
}

func applyProbeToHost(h *MachineHost, res *ProbeResult) {
	h.LastProbedAt = res.CheckedAt
	h.Facts = res.Facts
	h.StatusMessage = res.Message
	h.Issues = res.Issues
	switch {
	case res.AuthOK && res.LibvirtReady:
		h.Status = StatusReachable // green — ready to orchestrate
	case res.AuthOK:
		h.Status = StatusPartial // yellow — SSH OK, missing libs / services
	default:
		h.Status = StatusUnreachable // red — no TCP or SSH auth
	}
}

func probeHost(h *MachineHost) *ProbeResult {
	now := time.Now().UTC().Format(time.RFC3339)
	res := &ProbeResult{
		Facts:     map[string]string{},
		Issues:    []ProbeIssue{},
		CheckedAt: now,
	}
	host := h.EffectiveSSHHost()
	addr := fmt.Sprintf("%s:%d", host, h.SSHPort)
	if h.Stretched {
		res.Facts["reachability"] = "stretched"
		res.Facts["stretchedHost"] = host
		res.Facts["lanHost"] = h.SSHHost
	} else {
		res.Facts["reachability"] = "lan"
	}
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		hint := ""
		if !h.Stretched && strings.TrimSpace(h.StretchedHost) != "" {
			hint = fmt.Sprintf(" (LAN unreachable from here — try Stretched VPN toggle → %s)", h.StretchedHost)
		}
		res.Message = fmt.Sprintf("TCP connect failed to %s: %v%s", addr, err, hint)
		res.Issues = append(res.Issues, ProbeIssue{
			ID: "tcp-unreachable", Severity: "error",
			Message: res.Message, Fixable: false,
		})
		return res
	}
	_ = conn.Close()
	res.Reachable = true

	keyPath, key, err := loadIdentity(h.IdentityFile)
	if err != nil {
		res.Message = fmt.Sprintf("host reachable but no SSH identity: %v", err)
		res.Issues = append(res.Issues, ProbeIssue{
			ID: "no-identity", Severity: "error",
			Message: res.Message, Fixable: false,
		})
		return res
	}
	res.Facts["identityFile"] = keyPath

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		res.Message = fmt.Sprintf("invalid private key %s: %v", keyPath, err)
		res.Issues = append(res.Issues, ProbeIssue{
			ID: "bad-identity", Severity: "error",
			Message: res.Message, Fixable: false,
		})
		return res
	}

	cfg := &ssh.ClientConfig{
		User:            h.SSHUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // lab inventory — TODO known_hosts
		Timeout:         8 * time.Second,
	}
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		res.Message = fmt.Sprintf("SSH auth failed for %s: %v", h.Endpoint(), err)
		res.Issues = append(res.Issues, ProbeIssue{
			ID: "ssh-auth", Severity: "error",
			Message: res.Message, Fixable: false,
		})
		return res
	}
	defer client.Close()
	res.AuthOK = true

	run := func(cmd string) (string, error) {
		session, err := client.NewSession()
		if err != nil {
			return "", err
		}
		defer session.Close()
		var buf bytes.Buffer
		session.Stdout = &buf
		session.Stderr = &buf
		if err := session.Run(cmd); err != nil {
			return strings.TrimSpace(buf.String()), err
		}
		return strings.TrimSpace(buf.String()), nil
	}

	if out, err := run("hostname"); err == nil && out != "" {
		res.Facts["hostname"] = out
	}
	if out, err := run("cat /etc/os-release 2>/dev/null | grep -E '^(NAME|VERSION)=' | tr '\\n' ' '"); err == nil && out != "" {
		res.Facts["os"] = out
	}
	if out, err := run("uname -m"); err == nil && out != "" {
		res.Facts["arch"] = out
	}

	libvirtActive := false
	if out, err := run("systemctl is-active libvirtd 2>/dev/null || systemctl is-active libvirt 2>/dev/null || true"); err == nil {
		res.Facts["libvirtd"] = out
		if out == "active" {
			libvirtActive = true
		}
	}
	virshOK := false
	if out, err := run("command -v virsh >/dev/null && virsh list --name 2>/dev/null | wc -l || echo missing"); err == nil {
		res.Facts["virsh"] = out
		if out != "missing" && !strings.Contains(out, "missing") {
			virshOK = true
		}
	}
	if out, err := run("test -S /var/run/libvirt/libvirt-sock && echo yes || echo no"); err == nil {
		res.Facts["libvirtSocket"] = out
	}

	podmanOK := false
	if out, err := run("command -v podman >/dev/null && podman --version || echo missing"); err == nil {
		res.Facts["podman"] = out
		if out != "missing" && !strings.Contains(strings.ToLower(out), "missing") {
			podmanOK = true
			res.PodmanReady = true
		}
	}

	res.LibvirtReady = libvirtActive && virshOK
	if !res.LibvirtReady && libvirtActive && res.Facts["libvirtSocket"] == "yes" && virshOK {
		res.LibvirtReady = true
	}
	// Prefer active+virsh; socket+virsh also OK
	if !res.LibvirtReady {
		res.LibvirtReady = virshOK && (libvirtActive || res.Facts["libvirtSocket"] == "yes")
	}

	if !virshOK {
		res.Issues = append(res.Issues, ProbeIssue{
			ID: "virsh-missing", Severity: "error",
			Message:    "virsh / libvirt client packages missing — install libvirt stack on target",
			Fixable:   true,
			FixAction: FixInstallLibvirt,
		})
	} else if !libvirtActive {
		res.Issues = append(res.Issues, ProbeIssue{
			ID: "libvirtd-inactive", Severity: "error",
			Message:    "libvirtd is not active — enable and start the service",
			Fixable:   true,
			FixAction: FixStartLibvirtd,
		})
	}

	if !podmanOK {
		res.Issues = append(res.Issues, ProbeIssue{
			ID: "podman-missing", Severity: "warn",
			Message:    "podman missing — install for EE/runner agent container (deploy path)",
			Fixable:   true,
			FixAction: FixInstallPodman,
		})
	}

	// Green only when libvirt is ready. Podman is recommended but not required for "ready".
	res.Orchestration = res.AuthOK && res.LibvirtReady
	res.OK = res.Orchestration

	switch {
	case res.AuthOK && res.LibvirtReady:
		msg := fmt.Sprintf("SSH OK to %s — libvirt ready; can orchestrate against a plan.", h.Endpoint())
		if !podmanOK {
			msg += " (podman optional for EE agent — Fix this to install)"
		}
		res.Message = msg
	case res.AuthOK && !res.LibvirtReady:
		res.Message = fmt.Sprintf("SSH OK to %s — partial: libvirt/virsh not ready. Use Fix this to install/start on target.", h.Endpoint())
	default:
		res.Message = "probe incomplete"
	}
	return res
}

func loadIdentity(hint string) (string, []byte, error) {
	candidates := []string{}
	if hint != "" {
		candidates = append(candidates, expandHome(hint))
	}
	if v := os.Getenv("INVENTORY_SSH_KEY"); v != "" {
		candidates = append(candidates, expandHome(v))
	}
	if v := os.Getenv("SSH_IDENTITY_FILE"); v != "" {
		candidates = append(candidates, expandHome(v))
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		candidates = append(candidates,
			filepath.Join(home, ".ssh", "id_ecdsa"),
			filepath.Join(home, ".ssh", "id_ed25519"),
			filepath.Join(home, ".ssh", "id_rsa"),
		)
	}
	seen := map[string]bool{}
	var lastErr error
	for _, p := range candidates {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		b, err := os.ReadFile(p)
		if err != nil {
			lastErr = err
			continue
		}
		return p, b, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no identity file found (set identityFile or INVENTORY_SSH_KEY)")
	}
	return "", nil, lastErr
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		return filepath.Join(home, p[2:])
	}
	return p
}
