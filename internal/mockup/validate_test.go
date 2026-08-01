package mockup

import "testing"

func TestValidateTopology_DefaultOK(t *testing.T) {
	m := defaultMockUp("id", "rack", "lab.example.net", "libvirt", "", "now")
	r := ValidateTopology(m)
	if !r.OK {
		t.Fatalf("default rack should validate: %+v", r.Issues)
	}
	if r.PromoteSupported {
		t.Fatal("promote must stay stubbed false")
	}
}

func TestValidateTopology_OrphanVHost(t *testing.T) {
	m := defaultMockUp("id", "rack", "lab.example.net", "libvirt", "", "now")
	m.Spec.CanvasMode = "freeform"
	m.Spec.Canvas = &CanvasSpec{
		OmitHost: true, OmitGateway: true, OmitHub: true, OmitACM: true,
		Orphans: []CanvasNode{{ID: "ff-vh-1", Kind: "vhost", Label: "lonely-vhost"}},
	}
	m.Spec.Clusters = nil
	r := ValidateTopology(m)
	if r.OK {
		t.Fatal("orphan vHost should fail validate")
	}
	found := false
	for _, i := range r.Issues {
		if i.Code == "orphan-vhost-no-payload" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected orphan-vhost-no-payload, got %+v", r.Issues)
	}
}

func TestValidateTopology_ACMWithoutMgmt(t *testing.T) {
	m := defaultMockUp("id", "rack", "lab.example.net", "libvirt", "", "now")
	m.Spec.CanvasMode = "freeform"
	m.Spec.Canvas = &CanvasSpec{OmitHub: true}
	r := ValidateTopology(m)
	if r.OK {
		t.Fatal("ACM without mgmt should fail")
	}
	codes := map[string]bool{}
	for _, i := range r.Issues {
		codes[i.Code] = true
	}
	if !codes["acm-needs-mgmt"] {
		t.Fatalf("missing acm-needs-mgmt: %+v", r.Issues)
	}
}
