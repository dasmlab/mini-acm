package cheapcloud

import (
	"testing"

	"github.com/dasmlab/mock-me/internal/mockup"
)

func TestTargetsSingleSNO(t *testing.T) {
	m := &mockup.MockUp{}
	m.Spec.Style = mockup.StyleSingleSNOOCP
	tg := TargetsFromMockUp(m)
	if len(tg) != 1 || tg[0].Capability != "ocp-sno-slim" || tg[0].Count != 1 {
		t.Fatalf("%+v", tg)
	}
}

func TestTargetsACM(t *testing.T) {
	m := &mockup.MockUp{}
	m.Spec.Style = mockup.StyleACMMultiCluster
	m.Spec.Clusters = []mockup.ClusterNode{{ID: "c1"}, {ID: "c2"}}
	tg := TargetsFromMockUp(m)
	if tg[0].Count != 3 {
		t.Fatalf("want 3 (hub+2), got %d", tg[0].Count)
	}
}

func TestTargetsSurfing(t *testing.T) {
	m := &mockup.MockUp{}
	m.Spec.Style = mockup.StyleSurfingCdnR2
	tg := TargetsFromMockUp(m)
	if tg[0].Capability != "object-store" || tg[0].Provider != "r2" {
		t.Fatalf("%+v", tg)
	}
}
