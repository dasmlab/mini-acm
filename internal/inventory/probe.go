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

// Probe connects over SSH and smoke-checks readiness for orchestration.
func (s *Store) Probe(id string) (*ProbeResult, error) {
	h, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	res := probeHost(h)
	h.LastProbedAt = res.CheckedAt
	h.Facts = res.Facts
	h.StatusMessage = res.Message
	switch {
	case res.Orchestration:
		h.Status = StatusReachable
	case res.AuthOK:
		h.Status = StatusPartial
	default:
		h.Status = StatusUnreachable
	}
	_ = s.Save(h)
	res.Host = h
	return res, nil
}

func probeHost(h *MachineHost) *ProbeResult {
	now := time.Now().UTC().Format(time.RFC3339)
	res := &ProbeResult{
		Facts:     map[string]string{},
		CheckedAt: now,
	}
	addr := fmt.Sprintf("%s:%d", h.SSHHost, h.SSHPort)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		res.Message = fmt.Sprintf("TCP connect failed to %s: %v", addr, err)
		return res
	}
	_ = conn.Close()
	res.Reachable = true

	keyPath, key, err := loadIdentity(h.IdentityFile)
	if err != nil {
		res.Message = fmt.Sprintf("host reachable but no SSH identity: %v", err)
		return res
	}
	res.Facts["identityFile"] = keyPath

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		res.Message = fmt.Sprintf("invalid private key %s: %v", keyPath, err)
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

	// libvirt readiness — sufficient to start planning orchestration
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

	res.LibvirtReady = libvirtActive || (virshOK && res.Facts["libvirtSocket"] == "yes")
	res.Orchestration = res.AuthOK // SSH is enough to start planning; libvirt is a readiness signal
	res.OK = res.AuthOK

	switch {
	case res.AuthOK && res.LibvirtReady:
		res.Message = fmt.Sprintf("SSH OK to %s — libvirt ready; can orchestrate against a plan.", h.Endpoint())
	case res.AuthOK && !res.LibvirtReady:
		res.Message = fmt.Sprintf("SSH OK to %s — host reachable, but libvirt/virsh not ready yet (install/start libvirtd before deploy).", h.Endpoint())
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
