package deploy

import (
	"strings"
	"testing"

	"github.com/dasmlab/mock-me/internal/mockup"
)

func TestBuildGuestsIncludesHubAndSpokes(t *testing.T) {
	m := &mockup.MockUp{
		Metadata: mockup.Metadata{Name: "lab-rack-1"},
		Spec: mockup.Spec{
			Gateway: mockup.GatewayNode{ID: "gw1", Hostname: "vyos-lab-gw"},
			Hub:     mockup.HubNode{ID: "hub1", Hostname: "hub-sno", MAC: "52:54:00:13:00:20"},
			Clusters: []mockup.ClusterNode{
				{ID: "c1", Name: "cluster-dev01", Count: 2, MACPrefix: "52:54:00:20:01"},
				{ID: "c2", Name: "cluster-dev02", Count: 1, MACPrefix: "52:54:00:20:02"},
			},
		},
	}
	g := buildGuests(m)
	if len(g) != 1+1+2+1 {
		t.Fatalf("want 5 guests, got %d %#v", len(g), g)
	}
	script := ensureGuestsScript(m, g)
	for _, name := range []string{"vyos-lab-gw", "hub-sno", "cluster-dev01-0", "cluster-dev01-1", "cluster-dev02-0"} {
		if !strings.Contains(script, name) {
			t.Errorf("script missing guest %s", name)
		}
	}
	if !strings.Contains(script, "vol-create-as") || !strings.Contains(script, "virt-install") {
		t.Error("script should create volumes and call virt-install")
	}
}
