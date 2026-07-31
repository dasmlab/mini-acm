// Package mockup stores lab topology blueprints (MockUp = Target analogue).
package mockup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// Phase tracks wizard progress.
type Phase string

const (
	PhaseCreated    Phase = "created"
	PhaseConfigured Phase = "configured"
	PhaseHubReady   Phase = "hub-ready"
	PhaseACMReady   Phase = "acm-ready"
	PhaseClustered  Phase = "clustered"
	PhaseReady      Phase = "ready"
)

// MockUp is the top-level lab rack object (like a Target in etcd-synthetic-load).
type MockUp struct {
	APIVersion string   `json:"apiVersion" yaml:"apiVersion"`
	Kind       string   `json:"kind" yaml:"kind"`
	Metadata   Metadata `json:"metadata" yaml:"metadata"`
	Spec       Spec     `json:"spec" yaml:"spec"`
	Status     Status   `json:"status" yaml:"status"`
	Layout     Layout   `json:"layout" yaml:"layout"`
}

type Metadata struct {
	ID        string `json:"id" yaml:"id"`
	Name      string `json:"name" yaml:"name"`
	CreatedAt string `json:"createdAt" yaml:"createdAt"`
	UpdatedAt string `json:"updatedAt" yaml:"updatedAt"`
	Notes     string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type Spec struct {
	BaseDomain string        `json:"baseDomain" yaml:"baseDomain"`
	Provider   string        `json:"provider" yaml:"provider"`
	Network    NetworkSpec   `json:"network" yaml:"network"`
	Hub        HubNode       `json:"hub" yaml:"hub"`
	ACM        ACMNode       `json:"acm" yaml:"acm"`
	Clusters   []ClusterNode `json:"clusters" yaml:"clusters"`
	Gaps       GapParams     `json:"gaps" yaml:"gaps"` // MVP gap fields captured in UI
}

type NetworkSpec struct {
	MachineCIDR string `json:"machineCIDR" yaml:"machineCIDR"`
	Gateway     string `json:"gateway" yaml:"gateway"`
	APIVIP      string `json:"apiVIP" yaml:"apiVIP"`
	IngressVIP  string `json:"ingressVIP" yaml:"ingressVIP"`
	DHCPStart   string `json:"dhcpStart,omitempty" yaml:"dhcpStart,omitempty"`
	DHCPEnd     string `json:"dhcpEnd,omitempty" yaml:"dhcpEnd,omitempty"`
	DNS         string `json:"dns,omitempty" yaml:"dns,omitempty"`
}

type HubNode struct {
	ID         string `json:"id" yaml:"id"`
	Label      string `json:"label" yaml:"label"` // MGMT-CLUSTER
	Mode       string `json:"mode" yaml:"mode"`
	Version    string `json:"version" yaml:"version"`
	Profile    string `json:"profile" yaml:"profile"`
	Hostname   string `json:"hostname" yaml:"hostname"`
	IP         string `json:"ip" yaml:"ip"`
	MAC        string `json:"mac" yaml:"mac"`
	CPU        int    `json:"cpu" yaml:"cpu"`
	MemoryMiB  int    `json:"memoryMiB" yaml:"memoryMiB"`
	DiskGiB    int    `json:"diskGiB" yaml:"diskGiB"`
	InstallACM bool   `json:"installACM" yaml:"installACM"`
}

type ACMNode struct {
	ID         string `json:"id" yaml:"id"`
	Label      string `json:"label" yaml:"label"` // ACM
	Enabled    bool   `json:"enabled" yaml:"enabled"`
	MCEChannel string `json:"mceChannel" yaml:"mceChannel"`
	ACMChannel string `json:"acmChannel" yaml:"acmChannel"`
	Notes      string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type ClusterNode struct {
	ID         string `json:"id" yaml:"id"`
	Label      string `json:"label" yaml:"label"` // DEPLOYMENT-CLUSTER-X
	Name       string `json:"name" yaml:"name"`
	Version    string `json:"version" yaml:"version"`
	Profile    string `json:"profile" yaml:"profile"`
	Count      int    `json:"count" yaml:"count"`
	CPU        int    `json:"cpu" yaml:"cpu"`
	MemoryMiB  int    `json:"memoryMiB" yaml:"memoryMiB"`
	DiskGiB    int    `json:"diskGiB" yaml:"diskGiB"`
	IPBase     string `json:"ipBase" yaml:"ipBase"`
	MACPrefix  string `json:"macPrefix" yaml:"macPrefix"`
	APIVIP     string `json:"apiVIP" yaml:"apiVIP"`
	IngressVIP string `json:"ingressVIP" yaml:"ingressVIP"`
}

// GapParams captures MVP-gap inputs the UI must collect before unattended runs.
type GapParams struct {
	PullSecretFile   string `json:"pullSecretFile" yaml:"pullSecretFile"`
	SSHPublicKeyFile string `json:"sshPublicKeyFile" yaml:"sshPublicKeyFile"`
	ClusterImageSet  string `json:"clusterImageSet" yaml:"clusterImageSet"`
	DiscoveryISO     string `json:"discoveryISO" yaml:"discoveryISO"`
	HubKubeconfig    string `json:"hubKubeconfig" yaml:"hubKubeconfig"`
	ManualApprove    bool   `json:"manualApprove" yaml:"manualApprove"`
}

type Status struct {
	Phase   Phase  `json:"phase" yaml:"phase"`
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
}

// Layout stores SVG canvas positions (interview-me mind-map style).
type Layout struct {
	Nodes map[string]NodePos `json:"nodes" yaml:"nodes"`
}

type NodePos struct {
	X float64 `json:"x" yaml:"x"`
	Y float64 `json:"y" yaml:"y"`
}

// Store persists mockups under dataDir/mockups/<id>/mockup.yaml.
type Store struct {
	root string
}

func NewStore(dataDir string) (*Store, error) {
	root := filepath.Join(dataDir, "mockups")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &Store{root: root}, nil
}

type CreateReq struct {
	Name       string `json:"name"`
	BaseDomain string `json:"baseDomain"`
	Provider   string `json:"provider"`
	Notes      string `json:"notes"`
}

func (s *Store) Create(req CreateReq) (*MockUp, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("name required")
	}
	if req.BaseDomain == "" {
		req.BaseDomain = "lab.example.net"
	}
	if req.Provider == "" {
		req.Provider = "libvirt"
	}
	id := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339)
	m := defaultMockUp(id, req.Name, req.BaseDomain, req.Provider, req.Notes, now)
	if err := s.save(m); err != nil {
		return nil, err
	}
	return m, nil
}

func defaultMockUp(id, name, domain, provider, notes, now string) *MockUp {
	hubID := "hub"
	acmID := "acm"
	c0 := "cluster-0"
	m := &MockUp{
		APIVersion: "mini-acm.dasmlab.org/v1alpha1",
		Kind:       "MockUp",
		Metadata: Metadata{
			ID: id, Name: name, CreatedAt: now, UpdatedAt: now, Notes: notes,
		},
		Spec: Spec{
			BaseDomain: domain,
			Provider:   provider,
			Network: NetworkSpec{
				MachineCIDR: "192.168.130.0/24",
				Gateway:     "192.168.130.1",
				APIVIP:      "192.168.130.10",
				IngressVIP:  "192.168.130.11",
				DHCPStart:   "192.168.130.100",
				DHCPEnd:     "192.168.130.150",
				DNS:         "192.168.130.1",
			},
			Hub: HubNode{
				ID: hubID, Label: "MGMT-CLUSTER", Mode: "local-agent",
				Version: "4.18", Profile: "hub-supported",
				Hostname: "hub-sno", IP: "192.168.130.20", MAC: "52:54:00:13:00:20",
				CPU: 8, MemoryMiB: 24576, DiskGiB: 200, InstallACM: true,
			},
			ACM: ACMNode{
				ID: acmID, Label: "ACM", Enabled: true,
				MCEChannel: "stable-2.7", ACMChannel: "release-2.12",
			},
			Clusters: []ClusterNode{{
				ID: c0, Label: "DEPLOYMENT-CLUSTER-1", Name: "dev01",
				Version: "4.18", Profile: "supported", Count: 3,
				CPU: 4, MemoryMiB: 16384, DiskGiB: 120,
				IPBase: "192.168.130.21", MACPrefix: "52:54:00:13:00",
				APIVIP: "192.168.130.10", IngressVIP: "192.168.130.11",
			}},
			Gaps: GapParams{ManualApprove: true},
		},
		Status: Status{Phase: PhaseCreated, Message: "MockUp created — configure topology then run wizard"},
		Layout: Layout{Nodes: map[string]NodePos{
			hubID: {X: 320, Y: 280},
			acmID: {X: 320, Y: 140},
			c0:    {X: 520, Y: 280},
		}},
	}
	return m
}

func (s *Store) List() ([]*MockUp, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	var out []*MockUp
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		m, err := s.Get(e.Name())
		if err != nil {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

func (s *Store) Get(id string) (*MockUp, error) {
	b, err := os.ReadFile(s.path(id))
	if err != nil {
		return nil, err
	}
	var m MockUp
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *Store) Save(m *MockUp) error {
	m.Metadata.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return s.save(m)
}

func (s *Store) Delete(id string) error {
	return os.RemoveAll(filepath.Join(s.root, id))
}

func (s *Store) path(id string) string {
	return filepath.Join(s.root, id, "mockup.yaml")
}

func (s *Store) save(m *MockUp) error {
	dir := filepath.Join(s.root, m.Metadata.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(m.Metadata.ID), b, 0o644)
}

// Dir returns the mockup directory.
func (s *Store) Dir(id string) string {
	return filepath.Join(s.root, id)
}

// AddCluster appends a DEPLOYMENT-CLUSTER node.
func (m *MockUp) AddCluster() ClusterNode {
	n := len(m.Spec.Clusters) + 1
	id := fmt.Sprintf("cluster-%d", n-1)
	name := fmt.Sprintf("dev%02d", n)
	c := ClusterNode{
		ID: id, Label: fmt.Sprintf("DEPLOYMENT-CLUSTER-%d", n), Name: name,
		Version: m.Spec.Hub.Version, Profile: "supported", Count: 3,
		CPU: 4, MemoryMiB: 16384, DiskGiB: 120,
		IPBase:    fmt.Sprintf("192.168.130.%d", 21+((n-1)*10)),
		MACPrefix: "52:54:00:13:00",
		APIVIP:    m.Spec.Network.APIVIP, IngressVIP: m.Spec.Network.IngressVIP,
	}
	m.Spec.Clusters = append(m.Spec.Clusters, c)
	if m.Layout.Nodes == nil {
		m.Layout.Nodes = map[string]NodePos{}
	}
	m.Layout.Nodes[id] = NodePos{X: 520, Y: float64(120 + n*80)}
	return c
}

// ApplyProfileSizes updates CPU/RAM/disk from known profile names.
func ApplyHubProfile(h *HubNode) {
	switch h.Profile {
	case "hub-lab", "lab", "lab-tight":
		h.CPU, h.MemoryMiB, h.DiskGiB = 8, 16384, 160
	default:
		h.CPU, h.MemoryMiB, h.DiskGiB = 8, 24576, 200
	}
}

func ApplyClusterProfile(c *ClusterNode) {
	switch c.Profile {
	case "lab-small", "lab", "unsupported":
		c.CPU, c.MemoryMiB, c.DiskGiB = 4, 12288, 120
	default:
		c.CPU, c.MemoryMiB, c.DiskGiB = 4, 16384, 120
	}
}
