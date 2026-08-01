package mockup

import (
	"os"
	"testing"
)

func TestCreateDerive(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.Create(CreateReq{Name: "rack1", BaseDomain: "lab.example.net"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Spec.Hub.Label != "MGMT-CLUSTER" {
		t.Fatalf("hub label: %s", m.Spec.Hub.Label)
	}
	if m.Spec.InfraHost.Label != "INFRA-HOST" || m.Spec.InfraHost.ID == "" {
		t.Fatalf("infra host: %+v", m.Spec.InfraHost)
	}
	if m.Spec.InfraHost.Kind != "baremetal" {
		t.Fatalf("infra kind: %s", m.Spec.InfraHost.Kind)
	}
	if len(m.Spec.Clusters) != 2 {
		t.Fatalf("want 2 default clusters, got %d", len(m.Spec.Clusters))
	}
	if m.Spec.Clusters[0].ClusterImageSet == "" {
		t.Fatal("expected ClusterImageSet filled")
	}
	if m.Spec.Clusters[0].APIVIP == m.Spec.Clusters[1].APIVIP {
		t.Fatal("clusters should have distinct API VIPs")
	}
	paths, err := s.Derive(m.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(paths["infraHost"]); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(paths["hub"]); err != nil {
		t.Fatal(err)
	}
	cpath := paths["cluster-0"]
	if _, err := os.Stat(cpath); err != nil {
		t.Fatal(err)
	}
	_ = m.AddCluster()
	if len(m.Spec.Clusters) != 3 {
		t.Fatalf("after add: %d", len(m.Spec.Clusters))
	}
	if err := m.RemoveCluster(m.Spec.Clusters[2].ID); err != nil {
		t.Fatal(err)
	}
	if len(m.Spec.Clusters) != 2 {
		t.Fatalf("after remove: %d", len(m.Spec.Clusters))
	}
}

func TestImageSetName(t *testing.T) {
	if got := ImageSetName("4.18"); got != "img418-x86-64-appsub" {
		t.Fatalf("got %s", got)
	}
}
