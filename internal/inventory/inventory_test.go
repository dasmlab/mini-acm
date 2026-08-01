package inventory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureSeedAndList(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 seed, got %d", len(list))
	}
	if list[0].SSHHost != "192.168.1.142" || list[0].SSHUser != "dasm" || !list[0].Seed {
		t.Fatalf("bad seed: %+v", list[0])
	}
	if list[0].StretchedHost != "10.50.0.3" {
		t.Fatalf("want stretchedHost 10.50.0.3, got %q", list[0].StretchedHost)
	}
	if list[0].Stretched {
		t.Fatal("seed should default stretched=false (LAN); toggle on for cluster/VPN path")
	}
	if list[0].EffectiveSSHHost() != "192.168.1.142" {
		t.Fatalf("effective host: %s", list[0].EffectiveSSHHost())
	}
	list[0].Stretched = true
	if list[0].EffectiveSSHHost() != "10.50.0.3" {
		t.Fatalf("stretched effective host: %s", list[0].EffectiveSSHHost())
	}
	// second NewStore should not duplicate
	s2, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	list2, _ := s2.List()
	if len(list2) != 1 {
		t.Fatalf("seed duplicated: %d", len(list2))
	}
}

func TestCreateAndDelete(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	h, err := s.Create(CreateReq{Name: "extra", SSHUser: "root", SSHHost: "10.0.0.9"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(h.ID); err != nil {
		t.Fatal(err)
	}
	seed, _ := s.List()
	if err := s.Delete(seed[0].ID); err == nil {
		t.Fatal("expected seed delete to fail")
	}
}

func TestProbeSeedIfReachable(t *testing.T) {
	if os.Getenv("MINI_MOCK_PROBE_LIVE") != "1" {
		t.Skip("set MINI_MOCK_PROBE_LIVE=1 to run live SSH probe")
	}
	dir := t.TempDir()
	// use real home key
	home, _ := os.UserHomeDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	list, _ := s.List()
	list[0].IdentityFile = filepath.Join(home, ".ssh", "id_ecdsa")
	_ = s.Save(list[0])
	res, err := s.Probe(list[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if !res.AuthOK {
		t.Fatalf("expected auth OK: %+v", res)
	}
	t.Logf("probe: %s facts=%v", res.Message, res.Facts)
}
