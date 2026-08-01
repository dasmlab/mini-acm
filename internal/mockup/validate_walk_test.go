package mockup

import "testing"

func TestBuildValidateWalk_DefaultACM(t *testing.T) {
	m := defaultMockUp("id", "rack", "lab.example.net", "libvirt", "", "now")
	r := ValidateTopology(m)
	if len(r.Steps) < 5 {
		t.Fatalf("want several walk steps, got %d: %+v", len(r.Steps), r.Steps)
	}
	kinds := map[string]bool{}
	for _, s := range r.Steps {
		kinds[s.Kind] = true
		if s.Name == "" {
			t.Fatalf("empty name: %+v", s)
		}
	}
	for _, want := range []string{"MachineHost", "Adapter", "OCP-MGMT", "ACM", "OCP-DEPLOY"} {
		if !kinds[want] {
			t.Fatalf("missing kind %s in %#v", want, kinds)
		}
	}
}
