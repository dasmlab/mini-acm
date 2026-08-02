package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dasmlab/mock-me/internal/deploy"
	"github.com/dasmlab/mock-me/internal/inventory"
	"github.com/dasmlab/mock-me/internal/mockup"
)

const labE2EMockUpName = "lab-e2e"

func newLabE2ECmd() *cobra.Command {
	var (
		dataDir  string
		host     string
		user     string
		identity string
		timeout  time.Duration
		stretched bool
	)
	cmd := &cobra.Command{
		Use:   "lab-e2e",
		Short: "Headless lab system test: Deploy ACM Multi-Cluster through OCP-MGMT + ACM",
		Long: `Tear down prior lab-e2e leftovers, create a MockUp, run the Deploy engine in-process,
and exit 0 when OCP-MGMT kubeconfig works and MCE+ACM CSVs are Available.

Spokes remain out of scope (honest-block after ACM is OK for this harness).

Gate: set MOCK_ME_LAB_E2E=1 (make lab-e2e does this). Requires SSH to the seed host,
podman + mock-me-ee, writable /vm-disks/mock-me, and a real pull-secret
(MOCK_ME_DEV_PULL_SECRET / PULL_SECRET_FILE).`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if os.Getenv("MOCK_ME_LAB_E2E") != "1" {
				return fmt.Errorf("refusing to run: set MOCK_ME_LAB_E2E=1 (use: make lab-e2e)")
			}
			if dataDir == "" {
				dataDir = os.Getenv("DATA_DIR")
			}
			if dataDir == "" {
				dataDir = "/tmp/mock-me-labe2e"
			}
			if user == "" {
				user = "dasm"
			}
			if identity == "" {
				identity = os.Getenv("INVENTORY_SSH_KEY")
			}
			if identity == "" {
				identity = os.Getenv("SSH_IDENTITY_FILE")
			}
			if timeout <= 0 {
				timeout = 2 * time.Hour
			}
			return runLabE2E(labE2EOpts{
				dataDir:   dataDir,
				host:      host,
				user:      user,
				identity:  identity,
				timeout:   timeout,
				stretched: stretched,
			})
		},
	}
	cmd.Flags().StringVar(&dataDir, "data-dir", "", "local data dir (default /tmp/mock-me-labe2e)")
	cmd.Flags().StringVar(&host, "host", "", "inventory SSH host (default: seed LAN or stretched)")
	cmd.Flags().StringVar(&user, "user", "dasm", "SSH user")
	cmd.Flags().StringVar(&identity, "identity", "", "SSH private key path ($INVENTORY_SSH_KEY)")
	cmd.Flags().DurationVar(&timeout, "timeout", 2*time.Hour, "overall harness timeout")
	cmd.Flags().BoolVar(&stretched, "stretched", false, "use inventory stretched/VPN address when --host unset")
	return cmd
}

type labE2EOpts struct {
	dataDir   string
	host      string
	user      string
	identity  string
	timeout   time.Duration
	stretched bool
}

func runLabE2E(o labE2EOpts) error {
	start := time.Now()
	logf := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "lab-e2e: "+format+"\n", args...)
	}

	if err := os.MkdirAll(o.dataDir, 0o755); err != nil {
		return err
	}
	mockups, err := mockup.NewStore(o.dataDir)
	if err != nil {
		return err
	}
	inv, err := inventory.NewStore(o.dataDir)
	if err != nil {
		return err
	}
	eng := deploy.NewEngine(mockups, inv)

	host, err := pickLabHost(inv, o)
	if err != nil {
		return err
	}
	if o.identity != "" {
		host.IdentityFile = o.identity
		_ = inv.Save(host)
	}
	logf("host=%s endpoint=%s data=%s", host.Name, host.Endpoint(), o.dataDir)

	logf("preflight SSH + virsh + podman + /vm-disks/mock-me")
	if err := labPreflight(inv, host.ID); err != nil {
		return err
	}
	if err := requireRealPullSecret(); err != nil {
		return err
	}

	// Teardown prior lab-e2e mockups + known guest work dirs.
	if err := teardownPriorLabE2E(mockups, inv, eng, host.ID, logf); err != nil {
		logf("teardown warning: %v", err)
	}

	m, err := mockups.Create(mockup.CreateReq{
		Name:       labE2EMockUpName,
		BaseDomain: "lab.example.net",
		Provider:   "libvirt",
		Genre:      mockup.GenreClusterManagement,
		Style:      mockup.StyleACMMultiCluster,
		SeedDevLab: true,
		Notes:      "lab-e2e harness",
	})
	if err != nil {
		return fmt.Errorf("create mockup: %w", err)
	}
	logf("mockup id=%s name=%s", m.Metadata.ID, m.Metadata.Name)

	if err := requireRealPullSecretFile(m.Spec.Gaps.PullSecretFile); err != nil {
		return err
	}

	// Clear guests this MockUp owns (hub-sno etc.) left by prior UI deploys.
	logf("teardown guests for fresh lab-e2e")
	if out, err := eng.TeardownHost(m, host.ID); err != nil {
		logf("pre-start teardown: %v (%s)", err, truncateLab(out, 300))
	}

	job, err := eng.Start(m.Metadata.ID, host.ID, host.Name, host.Endpoint())
	if err != nil {
		return fmt.Errorf("deploy start: %w", err)
	}
	logf("deploy job=%s started", job.ID)

	deadline := start.Add(o.timeout)
	var final *deploy.Job
	for {
		if time.Now().After(deadline) {
			return failWithConsole(final, fmt.Errorf("harness timed out after %s", o.timeout))
		}
		j, err := eng.GetJob(m.Metadata.ID)
		if err != nil {
			return err
		}
		final = j
		if j == nil {
			time.Sleep(2 * time.Second)
			continue
		}
		if j.Status == deploy.JobRunning {
			if len(j.Console) > 0 {
				last := j.Console[len(j.Console)-1]
				logf("[%s] %s %s", last.At, last.Stage, truncateLab(last.Text, 160))
			}
			time.Sleep(15 * time.Second)
			continue
		}
		break
	}

	if err := assertLabE2E(final, m.Metadata.Name, o.dataDir, inv, host.ID); err != nil {
		return failWithConsole(final, err)
	}
	logf("PASS — OCP-MGMT + ACM in %s", time.Since(start).Round(time.Second))
	return nil
}

func pickLabHost(inv *inventory.Store, o labE2EOpts) (*inventory.MachineHost, error) {
	list, err := inv.List()
	if err != nil {
		return nil, err
	}
	var seed *inventory.MachineHost
	for _, h := range list {
		if h.Seed || h.Name == "lab-rhel10-seed" {
			seed = h
			break
		}
	}
	if seed == nil {
		if len(list) == 0 {
			return nil, fmt.Errorf("no inventory hosts")
		}
		seed = list[0]
	}
	if o.host != "" {
		seed.SSHHost = o.host
		seed.SSHUser = o.user
		seed.Stretched = false
		_ = inv.Save(seed)
		return seed, nil
	}
	if o.user != "" {
		seed.SSHUser = o.user
	}
	if o.stretched {
		seed.Stretched = true
		_ = inv.Save(seed)
	}
	return seed, nil
}

func labPreflight(inv *inventory.Store, hostID string) error {
	script := `set -eu
export LIBVIRT_DEFAULT_URI="${LIBVIRT_DEFAULT_URI:-qemu:///system}"
echo "whoami=$(whoami)"
command -v virsh >/dev/null
virsh list --all >/dev/null
command -v podman >/dev/null
test -d /vm-disks
test -w /vm-disks || test -w /vm-disks/mock-me
mkdir -p /vm-disks/mock-me/work /vm-disks/mock-me/images
echo PREFLIGHT_OK=1
`
	out, _, err := inv.RunScript(hostID, script)
	if err != nil {
		return fmt.Errorf("preflight: %v (%s)", err, truncateLab(out, 500))
	}
	if !strings.Contains(out, "PREFLIGHT_OK=1") {
		return fmt.Errorf("preflight incomplete: %s", truncateLab(out, 500))
	}
	return nil
}

func requireRealPullSecret() error {
	for _, c := range []string{
		os.Getenv("MOCK_ME_DEV_PULL_SECRET"),
		os.Getenv("PULL_SECRET_FILE"),
		os.Getenv("PULL_SECRET"),
	} {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if strings.HasPrefix(c, "{") {
			if strings.Contains(c, `"_mock_me"`) {
				continue
			}
			return nil
		}
		if b, err := os.ReadFile(c); err == nil && len(b) > 0 && !strings.Contains(string(b), `"_mock_me"`) {
			return nil
		}
	}
	for _, p := range []string{
		"/var/run/secrets/openshift/pull-secret",
		"/data/dev-lab/pull-secret.json",
	} {
		if b, err := os.ReadFile(p); err == nil && len(b) > 0 && !strings.Contains(string(b), `"_mock_me"`) {
			return nil
		}
	}
	return fmt.Errorf("real pull-secret required: set MOCK_ME_DEV_PULL_SECRET or PULL_SECRET_FILE to a path/JSON (not the DEV stub)")
}

func requireRealPullSecretFile(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("pull-secret %s: %w", path, err)
	}
	if strings.Contains(string(b), `"_mock_me"`) {
		return fmt.Errorf("pull-secret at %s is still the DEV stub — set MOCK_ME_DEV_PULL_SECRET and recreate", path)
	}
	return nil
}

func teardownPriorLabE2E(mockups *mockup.Store, inv *inventory.Store, eng *deploy.Engine, hostID string, logf func(string, ...any)) error {
	list, err := mockups.List()
	if err != nil {
		return err
	}
	for _, m := range list {
		if m.Metadata.Name != labE2EMockUpName && !strings.HasPrefix(m.Metadata.Name, "lab-e2e-") {
			continue
		}
		logf("teardown mockup %s (%s)", m.Metadata.Name, m.Metadata.ID)
		if _, err := eng.TeardownHost(m, hostID); err != nil {
			logf("host teardown: %v", err)
		}
		_ = mockups.Delete(m.Metadata.ID)
	}
	// Sweep orphan lab-e2e work dirs left after MockUp delete.
	script := `set -eu
export LIBVIRT_DEFAULT_URI="${LIBVIRT_DEFAULT_URI:-qemu:///system}"
for d in /vm-disks/mock-me/work/lab-e2e /vm-disks/mock-me/work/lab-e2e-*; do
  [ -e "$d" ] || continue
  rm -rf "$d"
  echo "RM $d"
done
echo TEARDOWN_SWEEP_OK=1
`
	out, _, err := inv.RunScript(hostID, script)
	if err != nil {
		return fmt.Errorf("sweep: %v (%s)", err, truncateLab(out, 400))
	}
	logf("sweep:\n%s", truncateLab(out, 800))
	return nil
}

func assertLabE2E(j *deploy.Job, mockupName, dataDir string, inv *inventory.Store, hostID string) error {
	if j == nil {
		return fmt.Errorf("no deploy job")
	}
	ocp := stageByID(j, deploy.StageOCP)
	acm := stageByID(j, deploy.StageACM)
	if ocp == nil || ocp.Status != deploy.StageOK {
		msg := ""
		if ocp != nil {
			msg = ocp.Message
		}
		return fmt.Errorf("OCP stage not ok (status=%v): %s", statusOf(ocp), msg)
	}
	if acm == nil || acm.Status != deploy.StageOK {
		msg := ""
		if acm != nil {
			msg = acm.Message
		}
		return fmt.Errorf("ACM stage not ok (status=%v): %s", statusOf(acm), msg)
	}

	kc := filepath.Join(dataDir, fmt.Sprintf("hub-%s", mockupName), "auth", "kubeconfig")
	st, err := os.Stat(kc)
	if err != nil || st.Size() < 32 {
		return fmt.Errorf("local kubeconfig missing/empty: %s", kc)
	}

	script := fmt.Sprintf(`set -eu
export LIBVIRT_DEFAULT_URI="${LIBVIRT_DEFAULT_URI:-qemu:///system}"
STATE=$(virsh --connect qemu:///system domstate hub-sno 2>/dev/null | tr -d '\n' || echo missing)
echo "HUB_STATE=$STATE"
test -d /vm-disks/mock-me/work/%s
ls /vm-disks/mock-me/work/%s/hub/agent*.iso >/dev/null 2>&1 \
  || ls /vm-disks/mock-me/images/hub-sno-agent.iso >/dev/null 2>&1 \
  || { echo "NO_ISO=1"; exit 1; }
echo ASSERT_HOST_OK=1
`, mockupName, mockupName)
	out, _, err := inv.RunScript(hostID, script)
	if err != nil {
		return fmt.Errorf("host assert: %v (%s)", err, truncateLab(out, 500))
	}
	if !strings.Contains(out, "ASSERT_HOST_OK=1") {
		return fmt.Errorf("host assert failed: %s", truncateLab(out, 500))
	}
	return nil
}

func stageByID(j *deploy.Job, id string) *deploy.Stage {
	for i := range j.Stages {
		if j.Stages[i].ID == id {
			return &j.Stages[i]
		}
	}
	return nil
}

func statusOf(s *deploy.Stage) string {
	if s == nil {
		return "missing"
	}
	return s.Status
}

func failWithConsole(j *deploy.Job, err error) error {
	if j == nil {
		return err
	}
	var b strings.Builder
	b.WriteString(err.Error())
	b.WriteString("\n--- deploy console (tail) ---\n")
	start := 0
	if len(j.Console) > 40 {
		start = len(j.Console) - 40
	}
	for _, line := range j.Console[start:] {
		fmt.Fprintf(&b, "%s [%s] %s\n", line.At, line.Stage, line.Text)
	}
	b.WriteString(fmt.Sprintf("--- job status=%s message=%s ---\n", j.Status, j.Message))
	return fmt.Errorf("%s", b.String())
}

func truncateLab(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
