package mockup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PhaseRank orders lifecycle phases for upgrade/downgrade guards.
func PhaseRank(p Phase) int {
	switch p {
	case PhaseCreated:
		return 10
	case PhaseConfigured, PhaseHubReady:
		return 20
	case PhaseValidated, PhaseACMReady:
		return 30
	case PhaseDeploying, PhaseClustered:
		return 40
	case PhaseDeployed, PhaseReady:
		return 50
	default:
		return 0
	}
}

// HasDerivedArtifacts reports whether Derive has written the hub plan YAML.
func (s *Store) HasDerivedArtifacts(id string) bool {
	hub := filepath.Join(s.Dir(id), "out", "hub.yaml")
	st, err := os.Stat(hub)
	return err == nil && !st.IsDir()
}

// ValidatePlan runs topology checks, soft gap warnings, and requires derived YAML.
// On success it advances status to validated (does not downgrade deployed).
func (s *Store) ValidatePlan(id string) (ValidationResult, *MockUp, error) {
	m, err := s.Get(id)
	if err != nil {
		return ValidationResult{}, nil, err
	}

	if !s.HasDerivedArtifacts(id) {
		if _, err := s.Derive(id); err != nil {
			return ValidationResult{}, nil, fmt.Errorf("derive before validate: %w", err)
		}
		m, err = s.Get(id)
		if err != nil {
			return ValidationResult{}, nil, err
		}
	}

	res := ValidateTopology(m)
	appendPlanWarnings(m, &res)
	// Recompute summary to mention plan/gaps.
	errs, warns := 0, 0
	for _, i := range res.Issues {
		if i.Severity == "error" {
			errs++
		} else {
			warns++
		}
	}
	res.OK = errs == 0
	if res.OK && warns == 0 {
		res.Summary = "Plan validated — topology + derived YAML look ready to deploy."
	} else if res.OK {
		res.Summary = fmt.Sprintf("Plan validated with %d warning(s) — deployable; fill real pull-secret/SSH paths before a real hub create.", warns)
	} else {
		res.Summary = fmt.Sprintf("%d error(s), %d warning(s) — fix before deploy.", errs, warns)
	}
	res.Steps = BuildValidateWalk(m, res.Issues)

	if res.OK {
		if PhaseRank(m.Status.Phase) < PhaseRank(PhaseValidated) {
			m.Status.Phase = PhaseValidated
			m.Status.Message = res.Summary
			if err := s.Save(m); err != nil {
				return res, m, err
			}
		} else if m.Status.Phase != PhaseDeploying && m.Status.Phase != PhaseDeployed {
			m.Status.Message = res.Summary
			_ = s.Save(m)
		}
	}
	return res, m, nil
}

func appendPlanWarnings(m *MockUp, res *ValidationResult) {
	gap := m.Spec.Gaps
	if isPlaceholderPath(gap.PullSecretFile) {
		res.Issues = append(res.Issues, ValidationIssue{
			Code: "gap-pull-secret", Severity: "warn", Object: "gaps",
			Message: "Pull secret path is still a placeholder — OK for lab validate; set a real file before hub create.",
		})
	}
	if isPlaceholderPath(gap.SSHPublicKeyFile) {
		res.Issues = append(res.Issues, ValidationIssue{
			Code: "gap-ssh-key", Severity: "warn", Object: "gaps",
			Message: "SSH public key path is still a placeholder — OK for lab validate; set a real file before hub create.",
		})
	}
	for _, c := range m.Spec.Clusters {
		if c.DiscoveryISO == "" || isPlaceholderPath(c.DiscoveryISO) {
			res.Issues = append(res.Issues, ValidationIssue{
				Code: "gap-discovery-iso", Severity: "warn", Object: c.ID,
				Message: fmt.Sprintf("%s discovery ISO not set yet — attach after InfraEnv produces an ISO.", c.Label),
			})
		}
	}
}

func isPlaceholderPath(p string) bool {
	p = strings.TrimSpace(p)
	return p == "" || strings.HasPrefix(p, "$")
}

// SetPhase updates MockUp status phase + message.
func (s *Store) SetPhase(id string, phase Phase, message string) (*MockUp, error) {
	m, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	m.Status.Phase = phase
	m.Status.Message = message
	if err := s.Save(m); err != nil {
		return nil, err
	}
	return m, nil
}

// LinkInventoryRef stores a MachineHost inventory id on the MACHINE-HOST object.
func (s *Store) LinkInventoryRef(id, inventoryID string) (*MockUp, error) {
	m, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	if inventoryID != "" && m.Spec.InfraHost.InventoryRef != inventoryID {
		m.Spec.InfraHost.InventoryRef = inventoryID
		if err := s.Save(m); err != nil {
			return nil, err
		}
	}
	return m, nil
}
