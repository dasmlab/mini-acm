package inventory

import "testing"

func TestApplyProbeStatusColors(t *testing.T) {
	h := &MachineHost{ID: "x", Name: "t"}

	applyProbeToHost(h, &ProbeResult{
		AuthOK: false, LibvirtReady: false,
		Message: "down", Issues: nil, Facts: map[string]string{}, CheckedAt: "t",
	})
	if h.Status != StatusUnreachable {
		t.Fatalf("want unreachable, got %s", h.Status)
	}

	applyProbeToHost(h, &ProbeResult{
		AuthOK: true, LibvirtReady: false,
		Message: "partial", Issues: []ProbeIssue{{ID: "virsh-missing", Fixable: true, FixAction: FixInstallLibvirt}},
		Facts: map[string]string{}, CheckedAt: "t",
	})
	if h.Status != StatusPartial {
		t.Fatalf("want partial, got %s", h.Status)
	}
	if len(h.Issues) != 1 || !h.Issues[0].Fixable {
		t.Fatalf("issues not stored: %+v", h.Issues)
	}

	applyProbeToHost(h, &ProbeResult{
		AuthOK: true, LibvirtReady: true,
		Message: "ready", Facts: map[string]string{}, CheckedAt: "t",
	})
	if h.Status != StatusReachable {
		t.Fatalf("want reachable, got %s", h.Status)
	}
}
