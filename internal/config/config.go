// Package config loads hub and managed-cluster YAML for mini-acm.
// Secrets (pull secret, SSH key, kubeconfig paths) come from env / files —
// never embed credentials in committed YAML.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Profile names match files under profiles/.
const (
	ProfileSupported    = "supported"
	ProfileLabSmall     = "lab-small"
	ProfileHubLab       = "hub-lab"
	ProfileHubSupported = "hub-supported"
)

// HubConfig drives Phase 1: local Agent-based Installer → SNO → ACM.
type HubConfig struct {
	APIVersion string       `yaml:"apiVersion"`
	Kind       string       `yaml:"kind"`
	Metadata   Meta         `yaml:"metadata"`
	Hub        HubSpec      `yaml:"hub"`
	Provider   ProviderSpec `yaml:"provider"`
	Network    NetworkSpec  `yaml:"network"`
	Node       NodeSpec     `yaml:"node"`
}

// ClusterConfig drives Phase 2: ACM lifecycle → compact 3-node cluster.
type ClusterConfig struct {
	APIVersion string       `yaml:"apiVersion"`
	Kind       string       `yaml:"kind"`
	Metadata   Meta         `yaml:"metadata"`
	Cluster    ClusterSpec  `yaml:"cluster"`
	Provider   ProviderSpec `yaml:"provider"`
	Network    NetworkSpec  `yaml:"network"`
	Nodes      NodesSpec    `yaml:"nodes"`
}

type Meta struct {
	Name  string `yaml:"name"`
	Notes string `yaml:"notes,omitempty"`
}

type HubSpec struct {
	// Mode: local-agent | existing-kubeconfig | demo-redhat (stub)
	Mode       string `yaml:"mode"`
	BaseDomain string `yaml:"baseDomain"`
	Version    string `yaml:"version"`
	Profile    string `yaml:"profile"`
	// WorkDir holds generated install-config, agent-config, ISO, auth.
	WorkDir string `yaml:"workDir"`
	// InstallACM after install-complete (default true when unset via ApplyDefaults).
	InstallACM *bool `yaml:"installACM,omitempty"`
}

type ClusterSpec struct {
	Name       string `yaml:"name"`
	BaseDomain string `yaml:"baseDomain"`
	Version    string `yaml:"version"`
	Profile    string `yaml:"profile"`
	// HubKubeconfig path; falls back to $KUBECONFIG / hub workdir auth.
	HubKubeconfig string `yaml:"hubKubeconfig,omitempty"`
}

type ProviderSpec struct {
	// Type: libvirt | stub | demo-redhat | ...
	Type        string `yaml:"type"`
	Network     string `yaml:"network,omitempty"`
	StoragePool string `yaml:"storagePool,omitempty"`
	URI         string `yaml:"uri,omitempty"`
}

type NetworkSpec struct {
	MachineCIDR string `yaml:"machineCIDR"`
	Gateway     string `yaml:"gateway"`
	APIVIP      string `yaml:"apiVIP"`
	IngressVIP  string `yaml:"ingressVIP"`
	DHCPStart   string `yaml:"dhcpStart,omitempty"`
	DHCPEnd     string `yaml:"dhcpEnd,omitempty"`
	DNS         string `yaml:"dns,omitempty"`
}

type NodeSpec struct {
	Hostname  string `yaml:"hostname"`
	Role      string `yaml:"role"`
	CPU       int    `yaml:"cpu"`
	MemoryMiB int    `yaml:"memoryMiB"`
	DiskGiB   int    `yaml:"diskGiB"`
	IP        string `yaml:"ip"`
	MAC       string `yaml:"mac,omitempty"`
}

type NodesSpec struct {
	Count     int    `yaml:"count"`
	Role      string `yaml:"role"`
	CPU       int    `yaml:"cpu"`
	MemoryMiB int    `yaml:"memoryMiB"`
	DiskGiB   int    `yaml:"diskGiB"`
	// IPBase is the first master IP; subsequent nodes increment last octet.
	IPBase string `yaml:"ipBase"`
	// MACPrefix e.g. 52:54:00:13:00 — last byte assigned per index.
	MACPrefix string `yaml:"macPrefix,omitempty"`
}

// LoadHub reads a hub YAML file.
func LoadHub(path string) (*HubConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c HubConfig
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse hub config: %w", err)
	}
	c.ApplyDefaults()
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// LoadCluster reads a managed-cluster YAML file.
func LoadCluster(path string) (*ClusterConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c ClusterConfig
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse cluster config: %w", err)
	}
	c.ApplyDefaults()
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *HubConfig) ApplyDefaults() {
	if c.APIVersion == "" {
		c.APIVersion = "mini-acm.dasmlab.org/v1alpha1"
	}
	if c.Kind == "" {
		c.Kind = "Hub"
	}
	if c.Hub.Mode == "" {
		c.Hub.Mode = "local-agent"
	}
	if c.Hub.Profile == "" {
		c.Hub.Profile = ProfileHubSupported
	}
	if c.Hub.WorkDir == "" && c.Metadata.Name != "" {
		c.Hub.WorkDir = fmt.Sprintf("./data/hub-%s", c.Metadata.Name)
	}
	if c.Hub.InstallACM == nil {
		t := true
		c.Hub.InstallACM = &t
	}
	if c.Provider.Type == "" {
		c.Provider.Type = "libvirt"
	}
	if c.Provider.Network == "" {
		c.Provider.Network = "ocp-lab"
	}
	if c.Provider.StoragePool == "" {
		c.Provider.StoragePool = "default"
	}
	if c.Network.DNS == "" {
		c.Network.DNS = c.Network.Gateway
	}
	if c.Node.Role == "" {
		c.Node.Role = "master"
	}
	applyHubProfile(&c.Node, c.Hub.Profile)
}

func (c *ClusterConfig) ApplyDefaults() {
	if c.APIVersion == "" {
		c.APIVersion = "mini-acm.dasmlab.org/v1alpha1"
	}
	if c.Kind == "" {
		c.Kind = "ManagedCluster"
	}
	if c.Cluster.Name == "" {
		c.Cluster.Name = c.Metadata.Name
	}
	if c.Cluster.Profile == "" {
		c.Cluster.Profile = ProfileSupported
	}
	if c.Provider.Type == "" {
		c.Provider.Type = "libvirt"
	}
	if c.Provider.Network == "" {
		c.Provider.Network = "ocp-lab"
	}
	if c.Provider.StoragePool == "" {
		c.Provider.StoragePool = "default"
	}
	if c.Network.DNS == "" {
		c.Network.DNS = c.Network.Gateway
	}
	if c.Nodes.Count == 0 {
		c.Nodes.Count = 3
	}
	if c.Nodes.Role == "" {
		c.Nodes.Role = "master"
	}
	if c.Nodes.MACPrefix == "" {
		c.Nodes.MACPrefix = "52:54:00:13:00"
	}
	applyCompactProfile(&c.Nodes, c.Cluster.Profile)
}

func applyHubProfile(n *NodeSpec, profile string) {
	switch profile {
	case ProfileHubLab, "lab", "lab-tight":
		if n.CPU == 0 {
			n.CPU = 8
		}
		if n.MemoryMiB == 0 {
			n.MemoryMiB = 16384
		}
		if n.DiskGiB == 0 {
			n.DiskGiB = 160
		}
	default: // hub-supported
		if n.CPU == 0 {
			n.CPU = 8
		}
		if n.MemoryMiB == 0 {
			n.MemoryMiB = 24576
		}
		if n.DiskGiB == 0 {
			n.DiskGiB = 200
		}
	}
}

func applyCompactProfile(n *NodesSpec, profile string) {
	switch profile {
	case ProfileLabSmall, "lab", "unsupported":
		if n.CPU == 0 {
			n.CPU = 4
		}
		if n.MemoryMiB == 0 {
			n.MemoryMiB = 12288
		}
		if n.DiskGiB == 0 {
			n.DiskGiB = 120
		}
	default: // supported
		if n.CPU == 0 {
			n.CPU = 4
		}
		if n.MemoryMiB == 0 {
			n.MemoryMiB = 16384
		}
		if n.DiskGiB == 0 {
			n.DiskGiB = 120
		}
	}
}

func (c *HubConfig) Validate() error {
	if c.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is required")
	}
	if c.Hub.BaseDomain == "" {
		return fmt.Errorf("hub.baseDomain is required")
	}
	if c.Hub.Version == "" {
		return fmt.Errorf("hub.version is required")
	}
	switch c.Hub.Mode {
	case "local-agent", "existing-kubeconfig", "demo-redhat":
	default:
		return fmt.Errorf("hub.mode %q not supported (local-agent|existing-kubeconfig|demo-redhat)", c.Hub.Mode)
	}
	if c.Provider.Type == "" {
		return fmt.Errorf("provider.type is required")
	}
	return nil
}

func (c *ClusterConfig) Validate() error {
	if c.Cluster.Name == "" {
		return fmt.Errorf("cluster.name (or metadata.name) is required")
	}
	if c.Cluster.BaseDomain == "" {
		return fmt.Errorf("cluster.baseDomain is required")
	}
	if c.Cluster.Version == "" {
		return fmt.Errorf("cluster.version is required")
	}
	if c.Nodes.Count != 3 {
		return fmt.Errorf("MVP only supports nodes.count=3 (compact); got %d", c.Nodes.Count)
	}
	if c.Network.MachineCIDR == "" || c.Network.APIVIP == "" || c.Network.IngressVIP == "" {
		return fmt.Errorf("network.machineCIDR, apiVIP, and ingressVIP are required")
	}
	return nil
}

// WantsACM reports whether ACM should be installed after hub OCP is up.
func (c *HubConfig) WantsACM() bool {
	return c.Hub.InstallACM == nil || *c.Hub.InstallACM
}

// IsLabProfile is true for intentionally undersized / unsupported profiles.
func IsLabProfile(profile string) bool {
	switch profile {
	case ProfileLabSmall, ProfileHubLab, "lab", "lab-tight", "unsupported":
		return true
	default:
		return false
	}
}
