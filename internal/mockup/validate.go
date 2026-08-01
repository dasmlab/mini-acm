package mockup

import "fmt"

// ValidationIssue is one topology problem (free-form teaching validate collects many).
type ValidationIssue struct {
	Code    string `json:"code"`
	Severity string `json:"severity"` // error | warn
	Object  string `json:"object,omitempty"`
	Message string `json:"message"`
}

// ValidationResult is the full pass over a MockUp topology.
type ValidationResult struct {
	OK       bool              `json:"ok"`
	Mode     string            `json:"mode"`
	Issues   []ValidationIssue `json:"issues"`
	Summary  string            `json:"summary"`
	// PromoteSupported is always false for now — free-form → guided is a later feature.
	PromoteSupported bool `json:"promoteSupported"`
}

// ValidateTopology checks the effective rack (respecting free-form omits + orphans).
// Collects as many issues as possible in one pass for live teaching demos.
func ValidateTopology(m *MockUp) ValidationResult {
	res := ValidationResult{
		Mode:             m.Spec.CanvasMode,
		PromoteSupported: false, // TODO(later): free-form → constrained promote
		Issues:           []ValidationIssue{},
	}
	if res.Mode == "" {
		res.Mode = "guided"
	}

	hasHost := m.EffectiveHost()
	hasGW := m.EffectiveGateway()
	hasHub := m.EffectiveHub()
	hasACM := m.EffectiveACM()
	clusters := m.Spec.Clusters
	if clusters == nil {
		clusters = []ClusterNode{}
	}
	style := m.Spec.Style
	if style == "" {
		style = StyleMiniACMMultiCluster
	}
	isSNOOnly := style == StyleSingleSNOOCP

	orphans := []CanvasNode{}
	if m.Spec.Canvas != nil {
		orphans = m.Spec.Canvas.Orphans
	}

	// --- style-specific minimum picture ---
	if isSNOOnly {
		if !hasHub {
			res.Issues = append(res.Issues, ValidationIssue{
				Code: "sno-needs-mgmt", Severity: "error",
				Message: "Single SNO style needs an OCP-MGMT (SNO) cluster — that is the whole point of this MockUp.",
			})
		}
		if hasACM {
			res.Issues = append(res.Issues, ValidationIssue{
				Code: "sno-unexpected-acm", Severity: "warn", Object: m.Spec.ACM.ID,
				Message: "ACM is enabled but this style stops before ACM — hide/disable ACM, or switch style to MINI ACM Multi-Cluster.",
			})
		}
		if len(clusters) > 0 {
			res.Issues = append(res.Issues, ValidationIssue{
				Code: "sno-unexpected-deploy", Severity: "warn",
				Message: "Deployment clusters are present on a Single SNO MockUp — remove them, or use MINI ACM Multi-Cluster.",
			})
		}
	} else {
		// MINI ACM (and default): ACM lab picture
		if hasACM && !hasHub {
			res.Issues = append(res.Issues, ValidationIssue{
				Code: "acm-needs-mgmt", Severity: "error", Object: m.Spec.ACM.ID,
				Message: "ACM is present but there is no OCP-MGMT — ACM must live on a home OCP (MGMT-CLUSTER).",
			})
		}
		if len(clusters) > 0 && !hasHub {
			res.Issues = append(res.Issues, ValidationIssue{
				Code: "deployments-need-mgmt", Severity: "error",
				Message: "OCP-DEPLOY cluster(s) exist without OCP-MGMT — you need at least mgmt + ACM to get a managed cluster off the ground.",
			})
		}
		if len(clusters) > 0 && !hasACM {
			res.Issues = append(res.Issues, ValidationIssue{
				Code: "deployments-need-acm", Severity: "error",
				Message: "OCP-DEPLOY cluster(s) exist but ACM is missing/disabled — spokes are managed by ACM on the mgmt cluster.",
			})
		}
		if hasHub && hasACM && len(clusters) == 0 {
			res.Issues = append(res.Issues, ValidationIssue{
				Code: "acm-needs-spoke", Severity: "error", Object: m.Spec.ACM.ID,
				Message: "ACM + OCP-MGMT are present but there is no OCP-DEPLOY — minimum demo needs ≥1 managed cluster.",
			})
		}
	}
	if !hasHost {
		res.Issues = append(res.Issues, ValidationIssue{
			Code: "missing-host", Severity: "warn",
			Message: "No MACHINE-HOST in the picture — in a real MockUp the adapter needs a host running libvirt/podman.",
		})
	}

	// --- vHosts need a payload (OCP-MGMT / OCP-DEPLOY / VyOS / HAP / other) ---
	coveredVHosts := map[string]string{} // vhost id → payload description

	if hasGW {
		coveredVHosts["vhost-gw"] = "VyOS (RTR/NF)"
	}
	if hasHub {
		coveredVHosts["vhost-hub-0"] = "OCP-MGMT (SNO)"
	}
	for _, c := range clusters {
		n := c.Count
		if n < 1 {
			n = 3
		}
		for i := 0; i < n; i++ {
			coveredVHosts[fmt.Sprintf("vhost-%s-%d", c.ID, i)] = fmt.Sprintf("%s (OCP node)", c.Label)
		}
	}
	for _, o := range orphans {
		if o.Kind != "appliance" {
			continue
		}
		if o.RunsOn == "" {
			res.Issues = append(res.Issues, ValidationIssue{
				Code: "appliance-no-vhost", Severity: "error", Object: o.ID,
				Message: fmt.Sprintf("%s appliance has no vHost underneath — drop/link it onto a vHost (middleware sits on a guest).", labelOf(o)),
			})
			continue
		}
		kind := o.ApplianceType
		if kind == "" {
			kind = "appliance"
		}
		coveredVHosts[o.RunsOn] = fmt.Sprintf("%s (%s)", labelOf(o), kind)
	}

	// Derived vHosts that should exist for present parents
	expectedDerived := []struct {
		id, role string
		present  bool
	}{
		{"vhost-gw", "GW/RTR", hasGW},
		{"vhost-hub-0", "MGMT OCP", hasHub},
	}
	for _, e := range expectedDerived {
		if !e.present {
			continue
		}
		if _, ok := coveredVHosts[e.id]; !ok {
			res.Issues = append(res.Issues, ValidationIssue{
				Code: "vhost-missing-payload", Severity: "error", Object: e.id,
				Message: fmt.Sprintf("vHost %s has no cluster/appliance on top — in a real MockUp this fails (missing %s payload).", e.id, e.role),
			})
		}
	}
	for _, c := range clusters {
		n := c.Count
		if n < 1 {
			n = 3
		}
		for i := 0; i < n; i++ {
			id := fmt.Sprintf("vhost-%s-%d", c.ID, i)
			if _, ok := coveredVHosts[id]; !ok {
				res.Issues = append(res.Issues, ValidationIssue{
					Code: "vhost-missing-payload", Severity: "error", Object: id,
					Message: fmt.Sprintf("vHost %s has no OCP/appliance payload — cluster %s expects this guest.", id, c.Label),
				})
			}
		}
	}

	// Orphan free-form vHosts must have something on top
	for _, o := range orphans {
		if o.Kind != "vhost" {
			continue
		}
		if _, ok := coveredVHosts[o.ID]; ok {
			continue
		}
		res.Issues = append(res.Issues, ValidationIssue{
			Code: "orphan-vhost-no-payload", Severity: "error", Object: o.ID,
			Message: fmt.Sprintf("In a real MockUp this would fail: vHost %q has no cluster/appliance (VyOS/HAP/other) on top — a vHost is middleware that must run something.", labelOf(o)),
		})
	}

	// Gateway without host
	if hasGW && !hasHost {
		res.Issues = append(res.Issues, ValidationIssue{
			Code: "gateway-needs-host", Severity: "warn", Object: m.Spec.Gateway.ID,
			Message: "VyOS gateway is shown without a MACHINE-HOST — the RTR guest still needs a host/adapter underneath.",
		})
	}

	errs := 0
	warns := 0
	for _, i := range res.Issues {
		if i.Severity == "error" {
			errs++
		} else {
			warns++
		}
	}
	res.OK = errs == 0
	if isSNOOnly {
		if res.OK && warns == 0 {
			res.Summary = "Topology looks complete for a Single SNO (OCP-MGMT) MockUp."
		} else if res.OK {
			res.Summary = fmt.Sprintf("OK with %d warning(s) — SNO picture is usable; tidy warns when hardening.", warns)
		} else {
			res.Summary = fmt.Sprintf("%d error(s), %d warning(s) — fix before treating this as a ready SNO plan.", errs, warns)
		}
	} else if res.OK && warns == 0 {
		res.Summary = "Topology looks complete for a minimal ACM lab picture."
	} else if res.OK {
		res.Summary = fmt.Sprintf("OK with %d warning(s) — good enough to teach; fix warns when you harden the rack.", warns)
	} else {
		res.Summary = fmt.Sprintf("%d error(s), %d warning(s) — fix errors before calling this a real MockUp.", errs, warns)
	}
	return res
}

func labelOf(o CanvasNode) string {
	if o.Label != "" {
		return o.Label
	}
	return o.ID
}
