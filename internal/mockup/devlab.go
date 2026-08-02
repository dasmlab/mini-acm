package mockup

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

const (
	devLabDirName = "dev-lab"
	devLabREADME  = "README-DEV-ONLY.txt"
	devPullSecret = "pull-secret.json"
	devSSHKey     = "id_ed25519"
	devSSHPub     = "id_ed25519.pub"
)

// SeedDevLabGaps writes throwaway DEV-only secrets under mockups/<id>/dev-lab/
// and points GapParams / discovery ISO paths at them. LAB/TEST/DEV ONLY —
// delete the MockUp (or the folder) to discard. Prefer real Wizard paths for anything serious.
func (s *Store) SeedDevLabGaps(id string) (*MockUp, error) {
	m, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(s.Dir(id), devLabDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}

	readme := strings.TrimSpace(`
DEV / LAB ONLY — throwaway click-through seeds
==============================================
Generated for hands-free "Use defaults" Validate → Deploy demos.

• SSH keypair and pull-secret path are NOT for production.
• Prefer filling real paths in the Wizard for any serious hub create.
• Losing these files is fine: delete the MockUp and recreate with defaults.
`) + "\n"
	_ = os.WriteFile(filepath.Join(dir, devLabREADME), []byte(readme), 0o644)

	privPath := filepath.Join(dir, devSSHKey)
	pubPath := filepath.Join(dir, devSSHPub)
	if _, err := os.Stat(pubPath); err != nil {
		if err := writeDevSSHKey(privPath, pubPath); err != nil {
			return nil, err
		}
	}

	pullPath := filepath.Join(dir, devPullSecret)
	needPull := true
	if b, err := os.ReadFile(pullPath); err == nil && len(b) > 0 {
		// Keep real secrets; refresh throwaway stubs when a better source appears.
		if !bytes.Contains(b, []byte(`"_mock_me"`)) {
			needPull = false
		}
	}
	if needPull {
		if err := writeDevPullSecret(pullPath); err != nil {
			return nil, err
		}
	}

	m.Spec.Gaps.SSHPublicKeyFile = pubPath
	m.Spec.Gaps.PullSecretFile = pullPath
	if m.Metadata.Notes == "" {
		m.Metadata.Notes = "DEV lab seeds under mockups/<id>/dev-lab (throwaway)"
	} else if !strings.Contains(m.Metadata.Notes, "dev-lab") {
		m.Metadata.Notes += " · DEV lab seeds under mockups/<id>/dev-lab (throwaway)"
	}

	for i := range m.Spec.Clusters {
		c := &m.Spec.Clusters[i]
		if !isPlaceholderPath(c.DiscoveryISO) && c.DiscoveryISO != "" {
			continue
		}
		name := c.Name
		if name == "" {
			name = fmt.Sprintf("cluster-%d", i+1)
		}
		isoPath := filepath.Join(dir, fmt.Sprintf("discovery-%s.iso", name))
		stub := fmt.Sprintf("DEV-ONLY stub ISO marker for %s — replace with InfraEnv isoDownloadURL after ACM.\n", name)
		if err := os.WriteFile(isoPath, []byte(stub), 0o644); err != nil {
			return nil, err
		}
		c.DiscoveryISO = isoPath
	}

	if err := s.Save(m); err != nil {
		return nil, err
	}
	return m, nil
}

func writeDevSSHKey(privPath, pubPath string) error {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	sshPriv, err := ssh.MarshalPrivateKey(priv, "mock-me DEV throwaway")
	if err != nil {
		// Fallback: OpenSSH wire format via Parse
		return fmt.Errorf("marshal ssh private key: %w", err)
	}
	if err := os.WriteFile(privPath, pem.EncodeToMemory(sshPriv), 0o600); err != nil {
		return err
	}
	pub, err := ssh.NewPublicKey(priv.Public().(ed25519.PublicKey))
	if err != nil {
		return err
	}
	auth := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub)))
	return os.WriteFile(pubPath, []byte(auth+" mock-me-dev-throwaway\n"), 0o644)
}

func writeDevPullSecret(path string) error {
	// Prefer a real secret from the environment when operators provide one.
	candidates := []string{
		os.Getenv("MOCK_ME_DEV_PULL_SECRET"),
		os.Getenv("PULL_SECRET_FILE"),
		os.Getenv("PULL_SECRET"),
	}
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		// Env may be a path or raw JSON.
		if strings.HasPrefix(c, "{") {
			return os.WriteFile(path, []byte(c+"\n"), 0o600)
		}
		if b, err := os.ReadFile(c); err == nil && len(b) > 0 {
			return os.WriteFile(path, b, 0o600)
		}
	}
	for _, p := range []string{
		"/var/run/secrets/openshift/pull-secret",
		"/data/dev-lab/pull-secret.json",
	} {
		if b, err := os.ReadFile(p); err == nil && len(b) > 0 {
			return os.WriteFile(path, b, 0o600)
		}
	}

	// Stub: valid docker-config JSON with valid base64 auth (credentials are fake).
	// Real registry pulls still fail — Wizard / MOCK_ME_DEV_PULL_SECRET for hub ISO.
	stub := `{
  "auths": {
    "registry.redhat.io": {
      "auth": "ZGV2OnRocm93YXdheQ==",
      "email": "dev-only@mock-me.local"
    },
    "cloud.openshift.com": {
      "auth": "ZGV2OnRocm93YXdheQ==",
      "email": "dev-only@mock-me.local"
    },
    "quay.io": {
      "auth": "ZGV2OnRocm93YXdheQ==",
      "email": "dev-only@mock-me.local"
    },
    "registry.connect.redhat.com": {
      "auth": "ZGV2OnRocm93YXdheQ==",
      "email": "dev-only@mock-me.local"
    }
  },
  "_mock_me": "DEV/LAB throwaway — replace with a real pull secret before hub create"
}
`
	return os.WriteFile(path, []byte(stub), 0o600)
}
