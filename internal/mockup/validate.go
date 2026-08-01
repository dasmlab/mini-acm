package mockup

import "fmt"

// ValidationIssue is one topology problem (free-form teaching validate collects many).
type ValidationIssue struct {
	Code     string `json:"code"`
	Severity string `json:"severity"` // error | warn
	Object   string `json:"object,omitempty"`
	Message  string `json:"message"`
}

// ValidationStep is one station in the validate walk (object + relational neighbours).
type ValidationStep struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"` // MachineHost | Adapter | VHost | …
	Name    string   `json:"name"`
	Icon    string   `json:"icon"`
	Relates []string `json:"relates,omitempty"` // human relation edges checked
	Status  string   `json:"status"`           // ok | warn | error
	Issue   string   `json:"issue,omitempty"`
}

// ValidationResult is the full pass over a MockUp topology.
type ValidationResult struct {
	OK       bool              `json:"ok"`
	Mode     string            `json:"mode"`
	Issues   []ValidationIssue `json:"issues"`
	Steps    []ValidationStep  `json:"steps,omitempty"`
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
		style = StyleACMMultiCluster
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
				Message: "ACM is enabled but this style stops before ACM — hide/disable ACM, or switch style to ACM Multi-Cluster.",
			})
		}
		if len(clusters) > 0 {
			res.Issues = append(res.Issues, ValidationIssue{
				Code: "sno-unexpected-deploy", Severity: "warn",
				Message: "Deployment clusters are present on a Single SNO MockUp — remove them, or use ACM Multi-Cluster.",
			})
		}
	} else {
		// mock-me (and default): ACM lab picture
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
	res.Steps = BuildValidateWalk(m, res.Issues)
	return res
}

func labelOf(o CanvasNode) string {
	if o.Label != "" {
		return o.Label
	}
	return o.ID
}

// BuildValidateWalk produces a dynamic checklist of objects + relational neighbours
// for the UI assembly-style validate animation.
func BuildValidateWalk(m *MockUp, issues []ValidationIssue) []ValidationStep {
	issueFor := func(id string) (sev, msg string) {
		sev = "ok"
		for _, iss := range issues {
			if iss.Object != "" && iss.Object != id {
				continue
			}
			// Prefer object-scoped; also match unlabeled style issues onto related kinds loosely.
			if iss.Object == id || iss.Object == "" {
				if iss.Severity == "error" {
					return "error", iss.Message
				}
				if sev != "error" {
					sev, msg = "warn", iss.Message
				}
			}
		}
		return sev, msg
	}
	// Tighter: only attach issues that name this object id.
	issueForID := func(id string) (sev, msg string) {
		sev = "ok"
		for _, iss := range issues {
			if iss.Object != id {
				continue
			}
			if iss.Severity == "error" {
				return "error", iss.Message
			}
			sev, msg = "warn", iss.Message
		}
		return sev, msg
	}
	_ = issueFor

	var steps []ValidationStep
	add := func(id, kind, name, icon string, relates []string) {
		sev, msg := issueForID(id)
		steps = append(steps, ValidationStep{
			ID: id, Kind: kind, Name: name, Icon: icon, Relates: relates, Status: sev, Issue: msg,
		})
	}

	hasHost := m.EffectiveHost()
	hasGW := m.EffectiveGateway()
	hasHub := m.EffectiveHub()
	hasACM := m.EffectiveACM()
	style := m.Spec.Style
	if style == "" {
		style = StyleACMMultiCluster
	}

	if hasHost {
		h := m.Spec.InfraHost
		name := h.Label
		if name == "" {
			name = h.Hostname
		}
		if name == "" {
			name = "MACHINE-HOST"
		}
		add(h.ID, "MachineHost", name, "dns", []string{"Adapter runsOn → MachineHost"})
		add("adapter-libvirt", "Adapter", "ADAPTER (libvirt)", "settings_ethernet", []string{
			"Adapter runsOn → " + name,
			"VHost hostedBy → Adapter",
		})
	} else {
		// Still show a missing-host step so the walk explains the gap.
		sev, msg := "warn", "No MACHINE-HOST in the picture"
		for _, iss := range issues {
			if iss.Code == "missing-host" {
				sev, msg = iss.Severity, iss.Message
				if sev == "error" {
					sev = "error"
				} else {
					sev = "warn"
				}
				break
			}
		}
		steps = append(steps, ValidationStep{
			ID: "missing-host", Kind: "MachineHost", Name: "(missing)", Icon: "dns",
			Relates: []string{"Adapter would runOn → MachineHost"}, Status: sev, Issue: msg,
		})
	}

	if hasGW {
		g := m.Spec.Gateway
		name := g.Label
		if name == "" {
			name = g.Hostname
		}
		add("vhost-gw", "VHost", "vHost-GW", "crop_square", []string{
			"VHost hostedBy → Adapter",
			"Gateway runsOn → vHost-GW",
		})
		add(g.ID, "Gateway", name, "router", []string{
			"Gateway runsOn → vHost-GW",
			"LAN guests ↔ VyOS edge",
		})
	}

	if hasHub {
		h := m.Spec.Hub
		name := h.Label
		if name == "" {
			name = h.Hostname
		}
		add("vhost-hub-0", "VHost", "vHost-MGMT", "crop_square", []string{
			"OCP-MGMT runsOn → vHost-MGMT",
		})
		add(h.ID, "OCP-MGMT", name, "memory", []string{
			"OCP-MGMT runsOn → vHost-MGMT",
			"ACM runsOn → OCP-MGMT",
		})
	}

	if hasACM {
		a := m.Spec.ACM
		name := a.Label
		if name == "" {
			name = "ACM"
		}
		rels := []string{"ACM runsOn → OCP-MGMT"}
		if style == StyleACMMultiCluster {
			rels = append(rels, "OCP-DEPLOY managedBy → ACM")
		}
		add(a.ID, "ACM", name, "extension", rels)
	}

	for _, c := range m.Spec.Clusters {
		n := c.Count
		if n < 1 {
			n = 3
		}
		name := c.Label
		if name == "" {
			name = c.Name
		}
		for i := 0; i < n; i++ {
			vid := fmt.Sprintf("vhost-%s-%d", c.ID, i)
			add(vid, "VHost", fmt.Sprintf("vHost-%s-%d", c.Name, i), "crop_square", []string{
				fmt.Sprintf("OCP-DEPLOY %s runsOn → %s", name, vid),
			})
		}
		add(c.ID, "OCP-DEPLOY", name, "developer_board", []string{
			fmt.Sprintf("%s runsOn → VHost×%d", name, n),
			fmt.Sprintf("%s managedBy → ACM", name),
		})
	}

	if m.Spec.Canvas != nil {
		for _, o := range m.Spec.Canvas.Orphans {
			kind := o.Kind
			icon := "help_outline"
			rels := []string{}
			if o.Kind == "vhost" {
				kind = "VHost"
				icon = "crop_square"
				rels = []string{"orphan VHost must host a payload (appliance/cluster)"}
			} else if o.Kind == "appliance" {
				kind = "Appliance"
				icon = "electrical_services"
				if o.RunsOn != "" {
					rels = append(rels, fmt.Sprintf("%s runsOn → %s", labelOf(o), o.RunsOn))
				} else {
					rels = append(rels, "appliance must runOn → VHost")
				}
			}
			add(o.ID, kind, labelOf(o), icon, rels)
		}
	}

	// Plan-level gap steps (no object id) — append once if present in issues.
	for _, iss := range issues {
		if iss.Object != "" {
			continue
		}
		if iss.Code == "gap-pull-secret" || iss.Code == "gap-ssh-key" || iss.Code == "gap-discovery-iso" ||
			iss.Code == "sno-needs-mgmt" || iss.Code == "sno-unexpected-acm" || iss.Code == "sno-unexpected-deploy" ||
			iss.Code == "deployments-need-mgmt" || iss.Code == "deployments-need-acm" || iss.Code == "acm-needs-spoke" {
			sev := "warn"
			if iss.Severity == "error" {
				sev = "error"
			}
			steps = append(steps, ValidationStep{
				ID: "rule-" + iss.Code, Kind: "Relation", Name: iss.Code,
				Icon: "hub", Relates: []string{iss.Message}, Status: sev, Issue: iss.Message,
			})
		}
	}

	if len(steps) == 0 {
		steps = append(steps, ValidationStep{
			ID: "empty", Kind: "Canvas", Name: "(empty rack)", Icon: "blur_on",
			Status: "warn", Issue: "Nothing to walk — add objects or use defaults",
		})
	}
	return steps
}
