package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadHubDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hub.yaml")
	content := `
apiVersion: mini-acm.dasmlab.org/v1alpha1
kind: Hub
metadata:
  name: hub
hub:
  mode: local-agent
  baseDomain: lab.example.net
  version: "4.18"
provider:
  type: libvirt
network:
  machineCIDR: 192.168.130.0/24
  gateway: 192.168.130.1
  apiVIP: 192.168.130.10
  ingressVIP: 192.168.130.11
node:
  hostname: hub-sno
  ip: 192.168.130.20
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadHub(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Node.CPU != 8 || c.Node.MemoryMiB != 24576 {
		t.Fatalf("expected hub-supported defaults, got cpu=%d mem=%d", c.Node.CPU, c.Node.MemoryMiB)
	}
	if c.Hub.WorkDir != "./data/hub-hub" {
		t.Fatalf("workDir: %s", c.Hub.WorkDir)
	}
	if !c.WantsACM() {
		t.Fatal("expected InstallACM default true")
	}
}

func TestLoadClusterLabSmall(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cluster.yaml")
	content := `
metadata:
  name: dev01
cluster:
  name: dev01
  baseDomain: lab.example.net
  version: "4.18"
  profile: lab-small
provider:
  type: libvirt
network:
  machineCIDR: 192.168.130.0/24
  gateway: 192.168.130.1
  apiVIP: 192.168.130.10
  ingressVIP: 192.168.130.11
nodes:
  count: 3
  ipBase: 192.168.130.21
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadCluster(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Nodes.MemoryMiB != 12288 {
		t.Fatalf("lab-small mem: %d", c.Nodes.MemoryMiB)
	}
	if !IsLabProfile(c.Cluster.Profile) {
		t.Fatal("expected lab profile")
	}
}

func TestClusterRejectsNonThree(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	content := `
metadata:
  name: x
cluster:
  name: x
  baseDomain: lab.example.net
  version: "4.18"
network:
  machineCIDR: 192.168.130.0/24
  gateway: 192.168.130.1
  apiVIP: 192.168.130.10
  ingressVIP: 192.168.130.11
nodes:
  count: 5
  ipBase: 192.168.130.21
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCluster(path); err == nil {
		t.Fatal("expected error for count!=3")
	}
}
