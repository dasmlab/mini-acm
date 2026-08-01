package mockup

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Derive writes hub + cluster example YAMLs under mockups/<id>/out/ for CLI use.
func (s *Store) Derive(id string) (map[string]string, error) {
	m, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	outDir := filepath.Join(s.Dir(id), "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	paths := map[string]string{}

	infraPath := filepath.Join(outDir, "infra-host.yaml")
	if err := writeYAML(infraPath, infraHostDoc(m)); err != nil {
		return nil, err
	}
	paths["infraHost"] = infraPath

	gwPath := filepath.Join(outDir, "gateway.yaml")
	if err := writeYAML(gwPath, gatewayDoc(m)); err != nil {
		return nil, err
	}
	paths["gateway"] = gwPath

	hubPath := filepath.Join(outDir, "hub.yaml")
	if err := writeYAML(hubPath, hubDoc(m)); err != nil {
		return nil, err
	}
	paths["hub"] = hubPath

	for i, c := range m.Spec.Clusters {
		p := filepath.Join(outDir, fmt.Sprintf("cluster-%s.yaml", c.Name))
		if err := writeYAML(p, clusterDoc(m, c)); err != nil {
			return nil, err
		}
		paths[fmt.Sprintf("cluster-%d", i)] = p
	}

	m.Status.Phase = PhaseConfigured
	m.Status.Message = "Derived hub/cluster YAML under out/"
	_ = s.Save(m)
	return paths, nil
}

func gatewayDoc(m *MockUp) map[string]any {
	g := m.Spec.Gateway
	return map[string]any{
		"apiVersion": "mini-acm.dasmlab.org/v1alpha1",
		"kind":       "Gateway",
		"metadata": map[string]any{
			"name":  g.Hostname,
			"notes": g.Notes,
			"labels": map[string]any{
				"mini-acm.dasmlab.org/role": "edge-router",
				"mini-acm.dasmlab.org/image": g.Image,
			},
		},
		"spec": map[string]any{
			"image": g.Image, "isoPath": g.ISOPath, "phase": g.Phase,
			"capacity": map[string]any{
				"cpu": g.CPU, "memoryMiB": g.MemoryMiB, "diskGiB": g.DiskGiB,
			},
			"disks": g.Disks,
			"nics":  g.NICs,
			"wan":   map[string]any{"bridge": g.WANBridge},
			"lan": map[string]any{
				"network": g.LANNetwork, "cidr": g.LANCIDR, "ip": g.LANIP,
			},
			"nat": g.NAT, "firewall": g.Firewall,
			"infraHost": m.Spec.InfraHost.Hostname,
			"hostsLabGuests": true,
		},
	}
}

func infraHostDoc(m *MockUp) map[string]any {
	h := m.Spec.InfraHost
	return map[string]any{
		"apiVersion": "mini-acm.dasmlab.org/v1alpha1",
		"kind":       "InfraHost",
		"metadata": map[string]any{
			"name":  h.Hostname,
			"notes": h.Notes,
			"labels": map[string]any{
				"mini-acm.dasmlab.org/role": "machine-host",
				"acm.reference":             "BareMetalHost-analogue",
			},
		},
		"spec": map[string]any{
			"kind": h.Kind, "hypervisor": h.Hypervisor, "os": h.OS, "arch": h.Arch,
			"capacity": map[string]any{
				"cpu": h.CPU, "memoryMiB": h.MemoryMiB, "diskGiB": h.DiskGiB,
			},
			"disks": h.Disks,
			"nics":  h.NICs,
			"runtime": map[string]any{
				"provider": m.Spec.Provider, "libvirtURI": h.LibvirtURI,
				"networkName": h.NetworkName, "storagePool": h.StoragePool,
				"podman": h.Podman,
			},
			"sshHost":      h.SSHHost,
			"acmReference": h.ACMReference,
			"hostsGuests":  true,
		},
	}
}

func hubDoc(m *MockUp) map[string]any {
	h := m.Spec.Hub
	ih := m.Spec.InfraHost
	netName := or(ih.NetworkName, "ocp-lab")
	pool := or(ih.StoragePool, "default")
	return map[string]any{
		"apiVersion": "mini-acm.dasmlab.org/v1alpha1",
		"kind":       "Hub",
		"metadata":   map[string]any{"name": m.Metadata.Name, "notes": m.Metadata.Notes},
		"hub": map[string]any{
			"mode": h.Mode, "baseDomain": m.Spec.BaseDomain, "version": h.Version,
			"profile": h.Profile, "workDir": fmt.Sprintf("./data/hub-%s", m.Metadata.Name),
			"installACM": h.InstallACM,
		},
		"provider": map[string]any{
			"type": m.Spec.Provider, "network": netName, "storagePool": pool,
			"infraHost": ih.Hostname,
		},
		"network": map[string]any{
			"machineCIDR": m.Spec.Network.MachineCIDR, "gateway": m.Spec.Network.Gateway,
			"apiVIP": m.Spec.Network.APIVIP, "ingressVIP": m.Spec.Network.IngressVIP,
			"dhcpStart": m.Spec.Network.DHCPStart, "dhcpEnd": m.Spec.Network.DHCPEnd,
			"dns": m.Spec.Network.DNS,
		},
		"node": map[string]any{
			"hostname": h.Hostname, "role": "master", "ip": h.IP, "mac": h.MAC,
			"cpu": h.CPU, "memoryMiB": h.MemoryMiB, "diskGiB": h.DiskGiB,
			"disks": h.Disks, "nics": h.NICs,
		},
	}
}

func clusterDoc(m *MockUp, c ClusterNode) map[string]any {
	notes := c.Notes
	if c.ClusterImageSet != "" {
		if notes != "" {
			notes += "; "
		}
		notes += "clusterImageSet=" + c.ClusterImageSet
	}
	if c.DiscoveryISO != "" {
		if notes != "" {
			notes += "; "
		}
		notes += "discoveryISO=" + c.DiscoveryISO
	}
	ih := m.Spec.InfraHost
	netName := or(ih.NetworkName, "ocp-lab")
	pool := or(ih.StoragePool, "default")
	return map[string]any{
		"apiVersion": "mini-acm.dasmlab.org/v1alpha1",
		"kind":       "ManagedCluster",
		"metadata":   map[string]any{"name": c.Name, "notes": notes},
		"cluster": map[string]any{
			"name": c.Name, "baseDomain": m.Spec.BaseDomain,
			"version": c.Version, "profile": c.Profile,
		},
		"provider": map[string]any{
			"type": m.Spec.Provider, "network": netName, "storagePool": pool,
			"infraHost": ih.Hostname,
		},
		"network": map[string]any{
			"machineCIDR": m.Spec.Network.MachineCIDR, "gateway": m.Spec.Network.Gateway,
			"apiVIP":     or(c.APIVIP, m.Spec.Network.APIVIP),
			"ingressVIP": or(c.IngressVIP, m.Spec.Network.IngressVIP),
			"dhcpStart":  m.Spec.Network.DHCPStart, "dhcpEnd": m.Spec.Network.DHCPEnd,
			"dns": m.Spec.Network.DNS,
		},
		"nodes": map[string]any{
			"count": c.Count, "role": "master",
			"cpu": c.CPU, "memoryMiB": c.MemoryMiB, "diskGiB": c.DiskGiB,
			"disks": c.Disks, "nics": c.NICs,
			"ipBase": c.IPBase, "macPrefix": c.MACPrefix,
		},
	}
}

func writeYAML(path string, v any) error {
	b, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
