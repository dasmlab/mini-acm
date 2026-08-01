package mockup

import "testing"

func TestValidatePlan_DefaultAdvances(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.Create(CreateReq{Name: "rack1"})
	if err != nil {
		t.Fatal(err)
	}
	res, got, err := s.ValidatePlan(m.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("want ok: %+v", res.Issues)
	}
	if got.Status.Phase != PhaseValidated {
		t.Fatalf("phase: %s", got.Status.Phase)
	}
	if !s.HasDerivedArtifacts(m.Metadata.ID) {
		t.Fatal("expected auto-derive")
	}
}

func TestPhaseRank(t *testing.T) {
	if PhaseRank(PhaseConfigured) >= PhaseRank(PhaseValidated) {
		t.Fatal("configured should rank below validated")
	}
	if PhaseRank(PhaseValidated) >= PhaseRank(PhaseDeployed) {
		t.Fatal("validated should rank below deployed")
	}
}
