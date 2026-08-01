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
	// InfraHost is the RHEL BM or nested VM that runs podman + libvirt and
	// slices hub/cluster guest VMs. Conceptual peer to ACM BareMetalHost /
	// provisioner host — not an OCP node itself.
	InfraHost InfraHostNode `json:"infraHost" yaml:"infraHost"`
	Hub       HubNode       `json:"hub" yaml:"hub"`
	ACM       ACMNode       `json:"acm" yaml:"acm"`
	Clusters  []ClusterNode `json:"clusters" yaml:"clusters"`
	Gaps      GapParams     `json:"gaps" yaml:"gaps"` // MVP gap fields captured in UI
}

// InfraHostNode is the lab machine that hosts libvirt guests for the ACM demo.
// Styling/naming echoes ACM BareMetalHost + Metal3 host inventory; platform
// for guest installs remains agentBareMetal via InfraEnv (not this object).
// Capacity includes multi-disk / multi-NIC inventory (outer vHOST or BM).
type InfraHostNode struct {
	ID          string `json:"id" yaml:"id"`
	Label       string `json:"label" yaml:"label"` // INFRA-HOST
	Hostname    string `json:"hostname" yaml:"hostname"`
	Kind        string `json:"kind" yaml:"kind"`               // baremetal | nested-vm
	Hypervisor  string `json:"hypervisor,omitempty" yaml:"hypervisor,omitempty"` // vmware | kvm | none
	OS          string `json:"os" yaml:"os"`                   // rhel-9 | rhel-10
	Arch        string `json:"arch,omitempty" yaml:"arch,omitempty"`
	CPU         int    `json:"cpu" yaml:"cpu"` // host capacity
	MemoryMiB   int    `json:"memoryMiB" yaml:"memoryMiB"`
	DiskGiB     int    `json:"diskGiB" yaml:"diskGiB"` // total; kept in sync with Disks
	Disks       []DiskSpec `json:"disks,omitempty" yaml:"disks,omitempty"`
	NICs        []NICSpec  `json:"nics,omitempty" yaml:"nics,omitempty"`
	LibvirtURI  string `json:"libvirtURI,omitempty" yaml:"libvirtURI,omitempty"`
	NetworkName string `json:"networkName,omitempty" yaml:"networkName,omitempty"`
	StoragePool string `json:"storagePool,omitempty" yaml:"storagePool,omitempty"`
	Podman      bool   `json:"podman" yaml:"podman"` // UI/serve often via podman
	SSHHost     string `json:"sshHost,omitempty" yaml:"sshHost,omitempty"`
	Notes       string `json:"notes,omitempty" yaml:"notes,omitempty"`
	ACMReference string `json:"acmReference,omitempty" yaml:"acmReference,omitempty"`
}

// DiskSpec describes one block device (vHOST inventory or guest VM disk).
type DiskSpec struct {
	Name    string `json:"name,omitempty" yaml:"name,omitempty"` // root, data0, …
	SizeGiB int    `json:"sizeGiB" yaml:"sizeGiB"`
	Bus     string `json:"bus,omitempty" yaml:"bus,omitempty"`   // nvme | virtio | sata
	Role    string `json:"role,omitempty" yaml:"role,omitempty"` // system | data | pool
}

// NICSpec describes one network interface (flat/bridged MVP: usually one).
type NICSpec struct {
	Name    string `json:"name,omitempty" yaml:"name,omitempty"`       // eth0, ens192
	Model   string `json:"model,omitempty" yaml:"model,omitempty"`     // virtio | e1000e
	Mode    string `json:"mode,omitempty" yaml:"mode,omitempty"`       // bridged | nat | isolated | libvirt-network
	Network string `json:"network,omitempty" yaml:"network,omitempty"` // bridge or libvirt net name
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
	ID         string     `json:"id" yaml:"id"`
	Label      string     `json:"label" yaml:"label"` // MGMT-CLUSTER
	Mode       string     `json:"mode" yaml:"mode"`
	Version    string     `json:"version" yaml:"version"`
	Profile    string     `json:"profile" yaml:"profile"`
	Hostname   string     `json:"hostname" yaml:"hostname"`
	IP         string     `json:"ip" yaml:"ip"`
	MAC        string     `json:"mac" yaml:"mac"`
	CPU        int        `json:"cpu" yaml:"cpu"`
	MemoryMiB  int        `json:"memoryMiB" yaml:"memoryMiB"`
	DiskGiB    int        `json:"diskGiB" yaml:"diskGiB"`
	Disks      []DiskSpec `json:"disks,omitempty" yaml:"disks,omitempty"`
	NICs       []NICSpec  `json:"nics,omitempty" yaml:"nics,omitempty"`
	InstallACM bool       `json:"installACM" yaml:"installACM"`
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
	ID         string     `json:"id" yaml:"id"`
	Label      string     `json:"label" yaml:"label"` // DEPLOYMENT-CLUSTER-X
	Name       string     `json:"name" yaml:"name"`
	Version    string     `json:"version" yaml:"version"`
	Profile    string     `json:"profile" yaml:"profile"`
	Count      int        `json:"count" yaml:"count"`
	CPU        int        `json:"cpu" yaml:"cpu"`
	MemoryMiB  int        `json:"memoryMiB" yaml:"memoryMiB"`
	DiskGiB    int        `json:"diskGiB" yaml:"diskGiB"`
	Disks      []DiskSpec `json:"disks,omitempty" yaml:"disks,omitempty"`
	NICs       []NICSpec  `json:"nics,omitempty" yaml:"nics,omitempty"`
	IPBase     string     `json:"ipBase" yaml:"ipBase"`
	MACPrefix  string     `json:"macPrefix" yaml:"macPrefix"`
	APIVIP     string     `json:"apiVIP" yaml:"apiVIP"`
	IngressVIP string     `json:"ingressVIP" yaml:"ingressVIP"`
	// Per-cluster lifecycle gaps / status (each DEPLOYMENT-CLUSTER is its own object).
	Phase           string `json:"phase,omitempty" yaml:"phase,omitempty"` // planned | created | installing | ready | destroy
	ClusterImageSet string `json:"clusterImageSet,omitempty" yaml:"clusterImageSet,omitempty"`
	DiscoveryISO    string `json:"discoveryISO,omitempty" yaml:"discoveryISO,omitempty"`
	Notes           string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

// GapParams captures shared hub MVP-gap inputs (cluster-specific gaps live on ClusterNode).
type GapParams struct {
	PullSecretFile   string `json:"pullSecretFile" yaml:"pullSecretFile"`
	SSHPublicKeyFile string `json:"sshPublicKeyFile" yaml:"sshPublicKeyFile"`
	HubKubeconfig    string `json:"hubKubeconfig" yaml:"hubKubeconfig"`
	ManualApprove    bool   `json:"manualApprove" yaml:"manualApprove"`
	// Deprecated shared fields — kept for older mockups; prefer ClusterNode fields.
	ClusterImageSet string `json:"clusterImageSet,omitempty" yaml:"clusterImageSet,omitempty"`
	DiscoveryISO    string `json:"discoveryISO,omitempty" yaml:"discoveryISO,omitempty"`
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
	infraID := "infra-host"
	hubID := "hub"
	acmID := "acm"
	c0 := newClusterNode(0, "4.18", "192.168.130.10", "192.168.130.11")
	c1 := newClusterNode(1, "4.18", "192.168.130.12", "192.168.130.13")
	return &MockUp{
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
			InfraHost: defaultInfraHost(name),
			Hub: HubNode{
				ID: hubID, Label: "MGMT-CLUSTER", Mode: "local-agent",
				Version: "4.18", Profile: "hub-supported",
				Hostname: "hub-sno", IP: "192.168.130.20", MAC: "52:54:00:13:00:20",
				CPU: 8, MemoryMiB: 24576, DiskGiB: 200, InstallACM: true,
				Disks: []DiskSpec{{Name: "vda", SizeGiB: 200, Bus: "virtio", Role: "system"}},
				NICs:  []NICSpec{{Name: "eth0", Model: "virtio", Mode: "libvirt-network", Network: "ocp-lab"}},
			},
			ACM: ACMNode{
				ID: acmID, Label: "ACM", Enabled: true,
				MCEChannel: "stable-2.7", ACMChannel: "release-2.12",
			},
			// Two clusters by default so multi-lifecycle is visible immediately.
			Clusters: []ClusterNode{c0, c1},
			Gaps: GapParams{
				ManualApprove:    true,
				PullSecretFile:   "$PULL_SECRET_FILE",
				SSHPublicKeyFile: "$SSH_PUBLIC_KEY_FILE",
				HubKubeconfig:    fmt.Sprintf("./data/hub-%s/auth/kubeconfig", name),
			},
		},
		Status: Status{Phase: PhaseCreated, Message: "MockUp created — add/edit clusters on Topology, fill gaps in Wizard"},
		Layout: Layout{Nodes: map[string]NodePos{
			infraID: {X: 120, Y: 460},
			hubID:   {X: 280, Y: 300},
			acmID:   {X: 280, Y: 160},
			c0.ID:   {X: 560, Y: 200},
			c1.ID:   {X: 560, Y: 340},
		}},
	}
}

func defaultInfraHost(rackName string) InfraHostNode {
	host := "rhel10-vhost-mini-acm"
	if rackName != "" {
		host = "prov-" + rackName
	}
	// Shaped like a nested RHEL 10 vHOST (e.g. VMware): multi-disk + single bridged NIC.
	disks := []DiskSpec{
		{Name: "nvme0", SizeGiB: 250, Bus: "nvme", Role: "system"},
		{Name: "nvme1", SizeGiB: 400, Bus: "nvme", Role: "pool"},
	}
	return InfraHostNode{
		ID: "infra-host", Label: "INFRA-HOST",
		Hostname: host, Kind: "nested-vm", Hypervisor: "vmware",
		OS: "rhel-10", Arch: "x86_64",
		CPU: 24, MemoryMiB: 40960,
		Disks: disks, DiskGiB: sumDiskGiB(disks),
		NICs: []NICSpec{
			{Name: "ens192", Model: "e1000e", Mode: "bridged", Network: "bridged-auto"},
		},
		LibvirtURI: "qemu:///system", NetworkName: "ocp-lab", StoragePool: "default",
		Podman: true,
		ACMReference: "BareMetalHost / agentBareMetal — this host runs libvirt guests; guests install via InfraEnv",
		Notes: "Nested RHEL 10 vHOST (or BM): runs CLI + podman + libvirt; slices MGMT + DEPLOYMENT cluster VMs. Disks/NICs inventory informs capacity planning.",
	}
}

func newClusterNode(index int, version, apiVIP, ingressVIP string) ClusterNode {
	n := index + 1
	id := fmt.Sprintf("cluster-%d", index)
	disks := []DiskSpec{{Name: "vda", SizeGiB: 120, Bus: "virtio", Role: "system"}}
	return ClusterNode{
		ID: id, Label: fmt.Sprintf("DEPLOYMENT-CLUSTER-%d", n),
		Name:    fmt.Sprintf("dev%02d", n),
		Version: version, Profile: "supported", Count: 3,
		CPU: 4, MemoryMiB: 16384, DiskGiB: 120, Disks: disks,
		NICs: []NICSpec{
			{Name: "eth0", Model: "virtio", Mode: "libvirt-network", Network: "ocp-lab"},
		},
		IPBase:          fmt.Sprintf("192.168.130.%d", 21+(index*10)),
		MACPrefix:       fmt.Sprintf("52:54:00:%02x:00", 0x13+index),
		APIVIP:          apiVIP,
		IngressVIP:      ingressVIP,
		Phase:           "planned",
		ClusterImageSet: ImageSetName(version),
	}
}

func sumDiskGiB(disks []DiskSpec) int {
	total := 0
	for _, d := range disks {
		total += d.SizeGiB
	}
	return total
}

func ensureGuestDisksNICs(diskGiB int, network string) ([]DiskSpec, []NICSpec, int) {
	if diskGiB <= 0 {
		diskGiB = 120
	}
	if network == "" {
		network = "ocp-lab"
	}
	disks := []DiskSpec{{Name: "vda", SizeGiB: diskGiB, Bus: "virtio", Role: "system"}}
	nics := []NICSpec{{Name: "eth0", Model: "virtio", Mode: "libvirt-network", Network: network}}
	return disks, nics, diskGiB
}

// ImageSetName derives a conventional ClusterImageSet name from an OCP version.
func ImageSetName(version string) string {
	compact := strings.ReplaceAll(version, ".", "")
	if compact == "" {
		compact = "418"
	}
	return fmt.Sprintf("img%s-x86-64-appsub", compact)
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
	normalize(&m)
	return &m, nil
}

func (s *Store) Save(m *MockUp) error {
	normalize(m)
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

// normalize fills MVP-gap defaults and migrates legacy shared cluster gaps.
func normalize(m *MockUp) {
	if m.Spec.Gaps.PullSecretFile == "" {
		m.Spec.Gaps.PullSecretFile = "$PULL_SECRET_FILE"
	}
	if m.Spec.Gaps.SSHPublicKeyFile == "" {
		m.Spec.Gaps.SSHPublicKeyFile = "$SSH_PUBLIC_KEY_FILE"
	}
	if m.Spec.Gaps.HubKubeconfig == "" && m.Metadata.Name != "" {
		m.Spec.Gaps.HubKubeconfig = fmt.Sprintf("./data/hub-%s/auth/kubeconfig", m.Metadata.Name)
	}
	if m.Spec.InfraHost.ID == "" {
		m.Spec.InfraHost = defaultInfraHost(m.Metadata.Name)
		if m.Layout.Nodes == nil {
			m.Layout.Nodes = map[string]NodePos{}
		}
		if _, ok := m.Layout.Nodes[m.Spec.InfraHost.ID]; !ok {
			m.Layout.Nodes[m.Spec.InfraHost.ID] = NodePos{X: 120, Y: 460}
		}
	}
	ih := &m.Spec.InfraHost
	if ih.Label == "" {
		ih.Label = "INFRA-HOST"
	}
	if ih.Kind == "" {
		ih.Kind = "nested-vm"
	}
	if ih.Kind == "nested-vm" && ih.Hypervisor == "" {
		ih.Hypervisor = "vmware"
	}
	if ih.OS == "" {
		ih.OS = "rhel-10"
	}
	if ih.Arch == "" {
		ih.Arch = "x86_64"
	}
	if ih.LibvirtURI == "" {
		ih.LibvirtURI = "qemu:///system"
	}
	if ih.NetworkName == "" {
		ih.NetworkName = "ocp-lab"
	}
	if ih.StoragePool == "" {
		ih.StoragePool = "default"
	}
	if ih.ACMReference == "" {
		ih.ACMReference = "BareMetalHost / agentBareMetal — this host runs libvirt guests; guests install via InfraEnv"
	}
	if len(ih.Disks) == 0 {
		if ih.DiskGiB > 0 {
			ih.Disks = []DiskSpec{{Name: "disk0", SizeGiB: ih.DiskGiB, Bus: "virtio", Role: "system"}}
		} else {
			ih.Disks = []DiskSpec{
				{Name: "nvme0", SizeGiB: 250, Bus: "nvme", Role: "system"},
				{Name: "nvme1", SizeGiB: 400, Bus: "nvme", Role: "pool"},
			}
		}
	}
	ih.DiskGiB = sumDiskGiB(ih.Disks)
	if len(ih.NICs) == 0 {
		ih.NICs = []NICSpec{{Name: "ens192", Model: "virtio", Mode: "bridged", Network: "bridged-auto"}}
	}

	h := &m.Spec.Hub
	if len(h.Disks) == 0 || len(h.NICs) == 0 {
		disks, nics, total := ensureGuestDisksNICs(h.DiskGiB, m.Spec.InfraHost.NetworkName)
		if len(h.Disks) == 0 {
			h.Disks = disks
			h.DiskGiB = total
		}
		if len(h.NICs) == 0 {
			h.NICs = nics
		}
	} else {
		h.DiskGiB = sumDiskGiB(h.Disks)
	}

	for i := range m.Spec.Clusters {
		c := &m.Spec.Clusters[i]
		if c.Phase == "" {
			c.Phase = "planned"
		}
		if c.ClusterImageSet == "" {
			if m.Spec.Gaps.ClusterImageSet != "" {
				c.ClusterImageSet = m.Spec.Gaps.ClusterImageSet
			} else {
				c.ClusterImageSet = ImageSetName(c.Version)
			}
		}
		if c.DiscoveryISO == "" && m.Spec.Gaps.DiscoveryISO != "" {
			c.DiscoveryISO = m.Spec.Gaps.DiscoveryISO
		}
		if c.Count == 0 {
			c.Count = 3
		}
		if len(c.Disks) == 0 || len(c.NICs) == 0 {
			disks, nics, total := ensureGuestDisksNICs(c.DiskGiB, m.Spec.InfraHost.NetworkName)
			if len(c.Disks) == 0 {
				c.Disks = disks
				c.DiskGiB = total
			}
			if len(c.NICs) == 0 {
				c.NICs = nics
			}
		} else {
			c.DiskGiB = sumDiskGiB(c.Disks)
		}
	}
}

// AddCluster appends a DEPLOYMENT-CLUSTER lifecycle node (VMs + ACM CRs).
func (m *MockUp) AddCluster() ClusterNode {
	index := nextClusterIndex(m.Spec.Clusters)
	apiOctet := 10 + index*2
	ingOctet := apiOctet + 1
	c := newClusterNode(
		index,
		m.Spec.Hub.Version,
		fmt.Sprintf("192.168.130.%d", apiOctet),
		fmt.Sprintf("192.168.130.%d", ingOctet),
	)
	m.Spec.Clusters = append(m.Spec.Clusters, c)
	if m.Layout.Nodes == nil {
		m.Layout.Nodes = map[string]NodePos{}
	}
	m.Layout.Nodes[c.ID] = NodePos{X: 480, Y: float64(160 + index*100)}
	return c
}

// RemoveCluster deletes a DEPLOYMENT-CLUSTER by id (underlying VM lifecycle object).
func (m *MockUp) RemoveCluster(clusterID string) error {
	if len(m.Spec.Clusters) <= 1 {
		return fmt.Errorf("keep at least one deployment cluster")
	}
	idx := -1
	for i, c := range m.Spec.Clusters {
		if c.ID == clusterID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("cluster %q not found", clusterID)
	}
	m.Spec.Clusters = append(m.Spec.Clusters[:idx], m.Spec.Clusters[idx+1:]...)
	if m.Layout.Nodes != nil {
		delete(m.Layout.Nodes, clusterID)
	}
	return nil
}

func nextClusterIndex(clusters []ClusterNode) int {
	used := map[int]bool{}
	for _, c := range clusters {
		var n int
		if _, err := fmt.Sscanf(c.ID, "cluster-%d", &n); err == nil {
			used[n] = true
		}
	}
	for i := 0; i < 64; i++ {
		if !used[i] {
			return i
		}
	}
	return len(clusters)
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
