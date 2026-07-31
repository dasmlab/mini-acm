package mockup

import (
	"os"
	"path/filepath"
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
	if len(m.Spec.Clusters) != 1 {
		t.Fatalf("clusters: %d", len(m.Spec.Clusters))
	}
	paths, err := s.Derive(m.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(paths["hub"]); err != nil {
		t.Fatal(err)
	}
	cpath := paths["cluster-0"]
	if !filepath.IsAbs(cpath) && filepath.Dir(cpath) == "." {
		t.Fatal("unexpected cluster path")
	}
	if _, err := os.Stat(cpath); err != nil {
		t.Fatal(err)
	}
	_ = m.AddCluster()
	if len(m.Spec.Clusters) != 2 {
		t.Fatalf("after add: %d", len(m.Spec.Clusters))
	}
}
