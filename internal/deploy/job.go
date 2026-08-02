package deploy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/dasmlab/mock-me/internal/inventory"
	"github.com/dasmlab/mock-me/internal/mockup"
)

// Stage IDs for the assembly-line UI.
const (
	StageGenerate = "generate"
	StageEE       = "ee"
	StageVInfra   = "vinfra"
	StageOCP      = "ocp"
	StageACM      = "acm"
	StageSpokes   = "spokes"
)

const (
	StagePending = "pending"
	StageRunning = "running"
	StageOK      = "ok"
	StageFailed  = "failed"
	StageBlocked = "blocked"
)

const (
	JobRunning   = "running"
	JobSucceeded = "succeeded"
	JobFailed    = "failed"
)

// Stage is one station on the deploy assembly line.
type Stage struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Detail  string `json:"detail"`
	Icon    string `json:"icon"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Log     string `json:"log,omitempty"`
}

// ConsoleLine is one breadcrumb in the deploy watch console (paths, host cmds, …).
type ConsoleLine struct {
	At    string `json:"at"`
	Stage string `json:"stage,omitempty"`
	Text  string `json:"text"`
}

// Job is an async deploy run for one MockUp.
type Job struct {
	ID           string        `json:"id"`
	MockUpID     string        `json:"mockUpId"`
	InventoryID  string        `json:"inventoryId"`
	HostName     string        `json:"hostName,omitempty"`
	HostEndpoint string        `json:"hostEndpoint,omitempty"`
	Status       string        `json:"status"`
	Message      string        `json:"message,omitempty"`
	Stages       []Stage       `json:"stages"`
	Console      []ConsoleLine `json:"console,omitempty"`
	StartedAt    string        `json:"startedAt"`
	UpdatedAt    string        `json:"updatedAt"`
	FinishedAt   string        `json:"finishedAt,omitempty"`
}

// Engine runs deploy jobs against inventory hosts.
type Engine struct {
	mockups *mockup.Store
	inv     *inventory.Store
	mu      sync.Mutex
	jobMu   sync.Mutex      // serializes deploy-job.json read/write
	active  map[string]bool // mockUpID → running
}

func NewEngine(mockups *mockup.Store, inv *inventory.Store) *Engine {
	return &Engine{
		mockups: mockups,
		inv:     inv,
		active:  map[string]bool{},
	}
}

func defaultStages() []Stage {
	return []Stage{
		{ID: StageEE, Label: "Execution environment", Detail: "Curated mock-me-ee via podman on inventory host", Icon: "precision_manufacturing", Status: StagePending},
		{ID: StageGenerate, Label: "Generate objects", Detail: "Stage YAML / secrets for the rack", Icon: "description", Status: StagePending},
		{ID: StageVInfra, Label: "Define lab VMs", Detail: "libvirt pool, net, gateway + MGMT + spoke domains", Icon: "lan", Status: StagePending},
		{ID: StageOCP, Label: "OCP-MGMT (SNO)", Detail: "Agent ISO → start hub VM (install still runs on host)", Icon: "memory", Status: StagePending},
		{ID: StageACM, Label: "Install ACM", Detail: "Operators on live MGMT kubeconfig", Icon: "extension", Status: StagePending},
		{ID: StageSpokes, Label: "OCP-DEPLOY clusters", Detail: "Discovery ISO + ACM spoke bring-up", Icon: "developer_board", Status: StagePending},
	}
}

func (e *Engine) jobPath(mockUpID string) string {
	return filepath.Join(e.mockups.Dir(mockUpID), "deploy-job.json")
}

func (e *Engine) SaveJob(j *Job) error {
	e.jobMu.Lock()
	defer e.jobMu.Unlock()
	j.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	b, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}
	path := e.jobPath(j.MockUpID)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (e *Engine) GetJob(mockUpID string) (*Job, error) {
	e.jobMu.Lock()
	defer e.jobMu.Unlock()
	b, err := os.ReadFile(e.jobPath(mockUpID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var j Job
	if err := json.Unmarshal(b, &j); err != nil {
		return nil, err
	}
	return &j, nil
}

// Start begins an async assembly-line deploy. Returns the initial job immediately.
func (e *Engine) Start(mockUpID, inventoryID, hostName, hostEndpoint string) (*Job, error) {
	m, err := e.mockups.Get(mockUpID)
	if err != nil {
		return nil, err
	}
	if err := m.RequireUnlocked(); err != nil {
		return nil, err
	}

	e.mu.Lock()
	if e.active[mockUpID] {
		e.mu.Unlock()
		if j, _ := e.GetJob(mockUpID); j != nil && j.Status == JobRunning {
			return j, nil
		}
		return nil, fmt.Errorf("deploy already running for this MockUp")
	}
	e.active[mockUpID] = true
	e.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	j := &Job{
		ID:           uuid.NewString(),
		MockUpID:     mockUpID,
		InventoryID:  inventoryID,
		HostName:     hostName,
		HostEndpoint: hostEndpoint,
		Status:       JobRunning,
		Message:      fmt.Sprintf("Assembly line started on %s (%s)", hostName, hostEndpoint),
		Stages:       defaultStages(),
		Console:      nil,
		StartedAt:    now,
		UpdatedAt:    now,
	}
	e.log(j, "", "assembly started → host=%s endpoint=%s mockup=%s", hostName, hostEndpoint, mockUpID)
	if err := e.SaveJob(j); err != nil {
		e.mu.Lock()
		delete(e.active, mockUpID)
		e.mu.Unlock()
		return nil, err
	}
	if _, err := e.mockups.SetPhase(mockUpID, mockup.PhaseDeploying, j.Message); err != nil {
		e.mu.Lock()
		delete(e.active, mockUpID)
		e.mu.Unlock()
		return nil, err
	}

	go e.run(j)
	return j, nil
}

func (e *Engine) log(j *Job, stage, format string, args ...any) {
	line := ConsoleLine{
		At:    time.Now().UTC().Format("15:04:05"),
		Stage: stage,
		Text:  fmt.Sprintf(format, args...),
	}
	j.Console = append(j.Console, line)
	const maxLines = 400
	if len(j.Console) > maxLines {
		j.Console = j.Console[len(j.Console)-maxLines:]
	}
}

func (e *Engine) finish(j *Job, status, msg string) {
	j.Status = status
	j.Message = msg
	j.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	e.log(j, "", "finished status=%s — %s", status, msg)
	_ = e.SaveJob(j)
	phase := mockup.PhaseFailed
	if status == JobSucceeded {
		phase = mockup.PhaseDeployed
	}
	_, _ = e.mockups.SetPhase(j.MockUpID, phase, msg)
	e.mu.Lock()
	delete(e.active, j.MockUpID)
	e.mu.Unlock()
}

func (e *Engine) setStage(j *Job, id, status, message, log string) {
	for i := range j.Stages {
		if j.Stages[i].ID == id {
			j.Stages[i].Status = status
			j.Stages[i].Message = message
			if log != "" {
				j.Stages[i].Log = truncate(log, 8000)
			}
			break
		}
	}
	switch status {
	case StageRunning:
		e.log(j, id, "▶ %s", message)
	case StageOK:
		e.log(j, id, "✓ %s", message)
	case StageFailed:
		e.log(j, id, "✗ %s", message)
	case StageBlocked:
		e.log(j, id, "⏸ %s", message)
	}
	_ = e.SaveJob(j)
}

func (e *Engine) run(j *Job) {
	defer func() {
		if r := recover(); r != nil {
			e.finish(j, JobFailed, fmt.Sprintf("deploy panicked: %v", r))
		}
	}()

	steps := []struct {
		id string
		fn func(*Job) error
	}{
		{StageEE, e.stageEE},
		{StageGenerate, e.stageGenerate},
		{StageVInfra, e.stageVInfra},
		{StageOCP, e.stageOCP},
		{StageACM, e.stageACM},
		{StageSpokes, e.stageSpokes},
	}

	for _, step := range steps {
		e.setStage(j, step.id, StageRunning, "In progress…", "")
		if err := step.fn(j); err != nil {
			msg := err.Error()
			status := StageFailed
			if isBlocked(err) {
				status = StageBlocked
			}
			e.setStage(j, step.id, status, msg, msg)
			prefix := "Stopped at"
			if status == StageBlocked {
				prefix = "Blocked at"
			}
			e.finish(j, JobFailed, fmt.Sprintf("%s %s: %s", prefix, step.id, msg))
			return
		}
	}
	e.finish(j, JobSucceeded, fmt.Sprintf("Assembly complete on %s — OCP-MGMT, ACM, and deploy clusters finished.", j.HostName))
}

type blockedError struct{ msg string }

func (e blockedError) Error() string { return e.msg }

func blocked(msg string) error { return blockedError{msg: msg} }

func isBlocked(err error) bool {
	_, ok := err.(blockedError)
	return ok
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// TeardownHost removes libvirt guests + volumes for the MockUp and deletes the remote work dir.
// Best-effort: local MockUp delete should still proceed if SSH fails.
func (e *Engine) TeardownHost(m *mockup.MockUp, inventoryID string) (string, error) {
	if m == nil || inventoryID == "" || e.inv == nil {
		return "", nil
	}
	guests := buildGuests(m)
	remoteRoot := remoteWorkRoot(m.Metadata.Name)
	script := destroyGuestsScript(m, guests, remoteRoot)
	out, _, err := e.inv.RunScript(inventoryID, script)
	if err != nil {
		return out, fmt.Errorf("host teardown: %v (%s)", err, truncate(out, 400))
	}
	return out, nil
}
