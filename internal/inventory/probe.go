package inventory

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
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
	addr := net.JoinHostPort(host, strconv.Itoa(h.SSHPort))
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

	// Single remote script (one SSH exec) + retries — multi-exec was flaky over WG/SSH.
	probeScript := `
set +e
HN=$(hostname 2>/dev/null)
OS=$(cat /etc/os-release 2>/dev/null | grep -E '^(NAME|VERSION)=' | tr '\n' ' ')
ARCH=$(uname -m 2>/dev/null)
LV=$(/usr/bin/systemctl is-active libvirtd 2>/dev/null)
[ "$LV" = "active" ] || LV=$(/usr/bin/systemctl is-active virtqemud 2>/dev/null)
[ "$LV" = "active" ] || LV=$(/usr/bin/systemctl is-active libvirt 2>/dev/null)
[ -n "$LV" ] || LV=unknown
if test -x /usr/bin/virsh; then
  if /usr/bin/virsh version >/dev/null 2>&1; then V=ok; else V=broken; fi
else
  V=missing
fi
if test -S /var/run/libvirt/libvirt-sock; then S=yes; else S=no; fi
if command -v podman >/dev/null 2>&1; then P=$(podman --version 2>/dev/null); else P=missing; fi
printf 'hostname=%s\n' "$HN"
printf 'os=%s\n' "$OS"
printf 'arch=%s\n' "$ARCH"
printf 'libvirtd=%s\n' "$LV"
printf 'virsh=%s\n' "$V"
printf 'socket=%s\n' "$S"
printf 'podman=%s\n' "$P"
printf 'PROBE_OK=1\n'
`

	libvirtActive := false
	virshOK := false
	sockOK := false
	podmanOK := false
	out, err := sshOutputRetry(client, probeScript, 3)
	if err != nil {
		res.Facts["libvirtProbeError"] = truncate(err.Error()+" "+out, 200)
	} else if !strings.Contains(out, "PROBE_OK=1") {
		res.Facts["libvirtProbeError"] = "incomplete probe output: " + truncate(out, 120)
	} else {
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			key, val, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			switch key {
			case "hostname":
				if val != "" {
					res.Facts["hostname"] = val
				}
			case "os":
				if val != "" {
					res.Facts["os"] = val
				}
			case "arch":
				if val != "" {
					res.Facts["arch"] = val
				}
			case "libvirtd":
				res.Facts["libvirtd"] = val
				libvirtActive = val == "active"
			case "virsh":
				res.Facts["virsh"] = val
				virshOK = val == "ok"
			case "socket":
				res.Facts["libvirtSocket"] = val
				sockOK = val == "yes"
			case "podman":
				res.Facts["podman"] = val
				if val != "" && val != "missing" && !strings.Contains(strings.ToLower(val), "missing") {
					podmanOK = true
					res.PodmanReady = true
				}
			}
		}
	}

	res.LibvirtReady = virshOK && (libvirtActive || sockOK)

	if !virshOK {
		res.Issues = append(res.Issues, ProbeIssue{
			ID: "virsh-missing", Severity: "error",
			Message:    "virsh / libvirt client packages missing — install libvirt stack on target",
			Fixable:   true,
			FixAction: FixInstallLibvirt,
		})
	} else if !libvirtActive && !sockOK {
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
