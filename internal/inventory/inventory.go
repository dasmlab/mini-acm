// Package inventory stores MACHINE-HOST orchestration targets (SSH inventory).
package inventory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

const (
	StatusUnknown     = "unknown"
	StatusReachable   = "reachable"   // green — SSH + libvirt ready to orchestrate
	StatusPartial     = "partial"     // yellow — SSH OK but missing libvirt/packages
	StatusUnreachable = "unreachable" // red — no TCP / SSH auth
)

// MachineHost is a physical/nested RHEL host that can run libvirtd + guests.
type MachineHost struct {
	ID           string `json:"id" yaml:"id"`
	Name         string `json:"name" yaml:"name"`
	SSHUser      string `json:"sshUser" yaml:"sshUser"`
	SSHHost      string `json:"sshHost" yaml:"sshHost"` // local / LAN address
	SSHPort      int    `json:"sshPort,omitempty" yaml:"sshPort,omitempty"`
	IdentityFile string `json:"identityFile,omitempty" yaml:"identityFile,omitempty"` // private key path — never key material
	Notes        string `json:"notes,omitempty" yaml:"notes,omitempty"`
	// Stretched: probe/fix via StretchedHost (e.g. WireGuard VPN) when the cluster
	// cannot reach the LAN address — optional boundary-crossing path, not a full VPN installer.
	Stretched     bool              `json:"stretched,omitempty" yaml:"stretched,omitempty"`
	StretchedHost string            `json:"stretchedHost,omitempty" yaml:"stretchedHost,omitempty"`
	Seed          bool              `json:"seed,omitempty" yaml:"seed,omitempty"`
	Status        string            `json:"status" yaml:"status"`
	StatusMessage string            `json:"statusMessage,omitempty" yaml:"statusMessage,omitempty"`
	LastProbedAt  string            `json:"lastProbedAt,omitempty" yaml:"lastProbedAt,omitempty"`
	Facts         map[string]string `json:"facts,omitempty" yaml:"facts,omitempty"`
	Issues        []ProbeIssue      `json:"issues,omitempty" yaml:"issues,omitempty"`
	CreatedAt     string            `json:"createdAt" yaml:"createdAt"`
	UpdatedAt     string            `json:"updatedAt" yaml:"updatedAt"`
}

// CreateReq creates a new inventory entry.
type CreateReq struct {
	Name          string `json:"name"`
	SSHUser       string `json:"sshUser"`
	SSHHost       string `json:"sshHost"`
	SSHPort       int    `json:"sshPort"`
	IdentityFile  string `json:"identityFile"`
	Notes         string `json:"notes"`
	Stretched     bool   `json:"stretched"`
	StretchedHost string `json:"stretchedHost"`
}

// ProbeResult is the outcome of an SSH (+ libvirt / podman) smoke check.
type ProbeResult struct {
	OK             bool              `json:"ok"`
	Reachable      bool              `json:"reachable"`
	AuthOK         bool              `json:"authOK"`
	LibvirtReady   bool              `json:"libvirtReady"`
	PodmanReady    bool              `json:"podmanReady"`
	InstallerReady bool              `json:"installerReady"` // openshift-install on PATH
	Orchestration  bool              `json:"orchestration"`  // green: ready to orchestrate a MockUp plan
	Message        string            `json:"message"`
	Facts          map[string]string `json:"facts,omitempty"`
	Issues         []ProbeIssue      `json:"issues,omitempty"`
	CheckedAt      string            `json:"checkedAt"`
	Host           *MachineHost      `json:"host,omitempty"`
}

// Store persists inventory under dataDir/inventory/<id>.yaml.
type Store struct {
	root string
}

func NewStore(dataDir string) (*Store, error) {
	root := filepath.Join(dataDir, "inventory")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	s := &Store{root: root}
	if err := s.ensureSeed(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) path(id string) string {
	return filepath.Join(s.root, id+".yaml")
}

const (
	seedLANIP = "192.168.1.142"
	seedVPNIP = "10.50.0.3" // WireGuard client on lab RHEL10 — use via Stretched toggle from cluster
)

func (s *Store) ensureSeed() error {
	list, err := s.List()
	if err != nil {
		return err
	}
	for _, h := range list {
		if h.Seed || h.SSHHost == seedLANIP {
			// Backfill stretched VPN address on existing seed without flipping the toggle.
			if h.StretchedHost == "" && (h.Seed || h.SSHHost == seedLANIP) {
				h.StretchedHost = seedVPNIP
				_ = s.save(h)
			}
			return nil
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	seed := &MachineHost{
		ID:            uuid.NewString(),
		Name:          "lab-rhel10-seed",
		SSHUser:       "dasm",
		SSHHost:       seedLANIP,
		SSHPort:       22,
		IdentityFile:  defaultIdentityHint(),
		Stretched:     false,
		StretchedHost: seedVPNIP,
		Notes:         "SEED — RHEL 10 MACHINE-HOST. LAN " + seedLANIP + "; optional stretched VPN " + seedVPNIP + " (toggle when cluster cannot reach LAN).",
		Seed:          true,
		Status:        StatusUnknown,
		StatusMessage: "not probed yet",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	return s.save(seed)
}

func defaultIdentityHint() string {
	if v := os.Getenv("INVENTORY_SSH_KEY"); v != "" {
		return v
	}
	if v := os.Getenv("SSH_IDENTITY_FILE"); v != "" {
		return v
	}
	for _, p := range []string{
		"/var/run/inventory-ssh/id_ecdsa",
		"/var/run/inventory-ssh/id_ed25519",
		"/var/run/inventory-ssh/id_rsa",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.ssh/id_ecdsa"
	}
	return filepath.Join(home, ".ssh", "id_ecdsa")
}

func (s *Store) List() ([]*MachineHost, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if os.IsNotExist(err) {
			return []*MachineHost{}, nil
		}
		return nil, err
	}
	var out []*MachineHost
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		h, err := s.Get(strings.TrimSuffix(e.Name(), ".yaml"))
		if err != nil {
			continue
		}
		out = append(out, h)
	}
	return out, nil
}

func (s *Store) Get(id string) (*MachineHost, error) {
	b, err := os.ReadFile(s.path(id))
	if err != nil {
		return nil, err
	}
	var h MachineHost
	if err := yaml.Unmarshal(b, &h); err != nil {
		return nil, err
	}
	normalizeHost(&h)
	return &h, nil
}

func (s *Store) Create(req CreateReq) (*MachineHost, error) {
	if strings.TrimSpace(req.SSHHost) == "" {
		return nil, fmt.Errorf("sshHost required")
	}
	if strings.TrimSpace(req.SSHUser) == "" {
		req.SSHUser = "dasm"
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = req.SSHUser + "@" + req.SSHHost
	}
	if req.SSHPort <= 0 {
		req.SSHPort = 22
	}
	if req.IdentityFile == "" {
		req.IdentityFile = defaultIdentityHint()
	}
	now := time.Now().UTC().Format(time.RFC3339)
	h := &MachineHost{
		ID:            uuid.NewString(),
		Name:          req.Name,
		SSHUser:       req.SSHUser,
		SSHHost:       req.SSHHost,
		SSHPort:       req.SSHPort,
		IdentityFile:  req.IdentityFile,
		Notes:         req.Notes,
		Stretched:     req.Stretched,
		StretchedHost: strings.TrimSpace(req.StretchedHost),
		Status:        StatusUnknown,
		StatusMessage: "not probed yet",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.save(h); err != nil {
		return nil, err
	}
	return h, nil
}

func (s *Store) Save(h *MachineHost) error {
	normalizeHost(h)
	h.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return s.save(h)
}

func (s *Store) Delete(id string) error {
	h, err := s.Get(id)
	if err != nil {
		return err
	}
	if h.Seed {
		return fmt.Errorf("cannot delete seed inventory entry — edit or reprobe instead")
	}
	return os.Remove(s.path(id))
}

func (s *Store) save(h *MachineHost) error {
	normalizeHost(h)
	b, err := yaml.Marshal(h)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(h.ID), b, 0o644)
}

func normalizeHost(h *MachineHost) {
	if h.SSHPort <= 0 {
		h.SSHPort = 22
	}
	if h.SSHUser == "" {
		h.SSHUser = "dasm"
	}
	if h.Status == "" {
		h.Status = StatusUnknown
	}
	if h.Facts == nil {
		h.Facts = map[string]string{}
	}
}

// EffectiveSSHHost is the address Probe/Fix dial — stretched VPN when toggled on.
func (h *MachineHost) EffectiveSSHHost() string {
	if h.Stretched && strings.TrimSpace(h.StretchedHost) != "" {
		return strings.TrimSpace(h.StretchedHost)
	}
	return h.SSHHost
}

// Endpoint returns user@effective-host for display / error messages.
func (h *MachineHost) Endpoint() string {
	return fmt.Sprintf("%s@%s", h.SSHUser, h.EffectiveSSHHost())
}
