// Package hub bootstraps the management SNO via local Agent-based Installer.
package hub

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/dasmlab/mini-mock/internal/acm"
	"github.com/dasmlab/mini-mock/internal/config"
	"github.com/dasmlab/mini-mock/internal/provider"
)

// Options control create behavior.
type Options struct {
	PullSecretPath string
	SSHKeyPath     string
	DryRun         bool
	Manual         bool
	SkipWait       bool
	SkipACM        bool
}

// Create runs Phase 1: generate agent assets, create VM, boot ISO, wait, optional ACM.
func Create(ctx context.Context, cfg *config.HubConfig, p provider.Provider, opts Options) error {
	if config.IsLabProfile(cfg.Hub.Profile) {
		fmt.Fprintln(os.Stderr, "warning: hub profile is lab/undersized — unsupported; ACM may OOM")
	}

	switch cfg.Hub.Mode {
	case "demo-redhat":
		return fmt.Errorf("hub.mode demo-redhat is a stub in MVP — see docs/ROADMAP.md")
	case "existing-kubeconfig":
		return existingHub(cfg, opts)
	case "local-agent":
		return localAgentCreate(ctx, cfg, p, opts)
	default:
		return fmt.Errorf("unsupported hub.mode %q", cfg.Hub.Mode)
	}
}

func existingHub(cfg *config.HubConfig, opts Options) error {
	fmt.Println("Using existing hub kubeconfig path from $KUBECONFIG / --kubeconfig")
	if opts.SkipACM || !cfg.WantsACM() {
		return nil
	}
	return acm.PrintInstallInstructions(cfg.Hub.WorkDir)
}

func localAgentCreate(ctx context.Context, cfg *config.HubConfig, p provider.Provider, opts Options) error {
	if err := os.MkdirAll(cfg.Hub.WorkDir, 0o755); err != nil {
		return err
	}

	pullSecret, err := readSecret(opts.PullSecretPath, "PULL_SECRET_FILE", "PULL_SECRET")
	if err != nil {
		if opts.DryRun || opts.Manual {
			fmt.Fprintf(os.Stderr, "warning: %v — using placeholder pull secret for dry-run/manual\n", err)
			pullSecret = `{"auths":{"cloud.openshift.com":{"auth":"PLACEHOLDER","email":"lab@example.com"}}}`
		} else {
			return fmt.Errorf("pull secret: %w", err)
		}
	}
	sshKey, err := readSSH(opts.SSHKeyPath)
	if err != nil {
		if opts.DryRun || opts.Manual {
			fmt.Fprintf(os.Stderr, "warning: %v — using placeholder SSH key for dry-run/manual\n", err)
			sshKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMiniMockDryRunPlaceholder lab@mini-mock"
		} else {
			return fmt.Errorf("ssh key: %w", err)
		}
	}

	mac := cfg.Node.MAC
	if mac == "" {
		mac = "52:54:00:13:00:20"
	}

	if err := writeInstallConfig(cfg, pullSecret, sshKey); err != nil {
		return err
	}
	if err := writeAgentConfig(cfg, mac); err != nil {
		return err
	}

	iso := filepath.Join(cfg.Hub.WorkDir, "agent.x86_64.iso")
	if err := createAgentImage(ctx, cfg.Hub.WorkDir, opts); err != nil {
		return err
	}

	net := provider.NetworkSpec{
		Name:      cfg.Provider.Network,
		CIDR:      cfg.Network.MachineCIDR,
		Gateway:   cfg.Network.Gateway,
		DHCPStart: or(cfg.Network.DHCPStart, "192.168.130.100"),
		DHCPEnd:   or(cfg.Network.DHCPEnd, "192.168.130.150"),
		Forward:   "nat",
	}
	if err := p.EnsureNetwork(ctx, net); err != nil {
		return err
	}

	node := provider.NodeSpec{
		Name:      cfg.Node.Hostname,
		CPU:       cfg.Node.CPU,
		MemoryMiB: cfg.Node.MemoryMiB,
		DiskGiB:   cfg.Node.DiskGiB,
		MAC:       mac,
		IP:        cfg.Node.IP,
		Network:   cfg.Provider.Network,
		Pool:      cfg.Provider.StoragePool,
		ISOPath:   iso,
	}
	if opts.DryRun || opts.Manual || !fileExists(iso) {
		node.ISOPath = ""
		fmt.Fprintf(os.Stdout, "ISO expected at %s (generate with openshift-install agent create image)\n", iso)
	}
	if err := p.CreateNode(ctx, node); err != nil {
		return err
	}
	if fileExists(iso) && !opts.DryRun {
		_ = p.AttachISO(ctx, node.Name, iso)
	}
	if err := p.StartNode(ctx, node.Name); err != nil {
		return err
	}

	printManualHubNext(cfg, iso, opts)

	if opts.SkipWait || opts.DryRun || opts.Manual {
		return nil
	}
	if err := waitInstall(ctx, cfg.Hub.WorkDir); err != nil {
		return err
	}
	if opts.SkipACM || !cfg.WantsACM() {
		return nil
	}
	return acm.Install(ctx, filepath.Join(cfg.Hub.WorkDir, "auth", "kubeconfig"))
}

// Status prints hub kubeconfig / node hints.
func Status(cfg *config.HubConfig) error {
	kc := filepath.Join(cfg.Hub.WorkDir, "auth", "kubeconfig")
	fmt.Printf("hub: %s\n", cfg.Metadata.Name)
	fmt.Printf("workDir: %s\n", cfg.Hub.WorkDir)
	fmt.Printf("kubeconfig: %s (exists=%v)\n", kc, fileExists(kc))
	fmt.Printf("mode: %s profile: %s\n", cfg.Hub.Mode, cfg.Hub.Profile)
	if fileExists(kc) {
		fmt.Printf("\nexport KUBECONFIG=%s\noc get nodes\noc get clusterversion\n", kc)
	}
	return nil
}

// Destroy removes the hub VM and optionally workdir.
func Destroy(ctx context.Context, cfg *config.HubConfig, p provider.Provider, purge bool) error {
	name := cfg.Node.Hostname
	if name == "" {
		name = cfg.Metadata.Name + "-sno"
	}
	_ = p.StopNode(ctx, name)
	if err := p.DeleteNode(ctx, name); err != nil {
		fmt.Fprintf(os.Stderr, "warning: delete node: %v\n", err)
	}
	if purge {
		fmt.Printf("removing workdir %s\n", cfg.Hub.WorkDir)
		return os.RemoveAll(cfg.Hub.WorkDir)
	}
	return nil
}

func writeInstallConfig(cfg *config.HubConfig, pullSecret, sshKey string) error {
	path := filepath.Join(cfg.Hub.WorkDir, "install-config.yaml")
	const tpl = `apiVersion: v1
baseDomain: {{.BaseDomain}}
metadata:
  name: {{.Name}}
controlPlane:
  name: master
  replicas: 1
  architecture: amd64
  hyperthreading: Enabled
compute:
- name: worker
  replicas: 0
  architecture: amd64
  hyperthreading: Enabled
networking:
  networkType: OVNKubernetes
  machineNetwork:
  - cidr: {{.MachineCIDR}}
  clusterNetwork:
  - cidr: 10.128.0.0/14
    hostPrefix: 23
  serviceNetwork:
  - 172.30.0.0/16
platform:
  none: {}
pullSecret: '{{.PullSecret}}'
sshKey: |
  {{.SSHKey}}
`
	t, err := template.New("ic").Parse(tpl)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	data := map[string]string{
		"BaseDomain":  cfg.Hub.BaseDomain,
		"Name":        cfg.Metadata.Name,
		"MachineCIDR": cfg.Network.MachineCIDR,
		"PullSecret":  sanitizePullSecret(pullSecret),
		"SSHKey":      strings.TrimSpace(sshKey),
	}
	return t.Execute(f, data)
}

func writeAgentConfig(cfg *config.HubConfig, mac string) error {
	path := filepath.Join(cfg.Hub.WorkDir, "agent-config.yaml")
	const tpl = `apiVersion: v1alpha1
kind: AgentConfig
metadata:
  name: {{.Name}}
rendezvousIP: {{.IP}}
hosts:
- hostname: {{.Hostname}}
  role: master
  interfaces:
  - name: eno1
    macAddress: "{{.MAC}}"
  networkConfig:
    interfaces:
    - name: eno1
      type: ethernet
      state: up
      mac-address: "{{.MAC}}"
      ipv4:
        enabled: true
        dhcp: false
        address:
        - ip: {{.IP}}
          prefix-length: 24
    dns-resolver:
      config:
        server:
        - {{.DNS}}
    routes:
      config:
      - destination: 0.0.0.0/0
        next-hop-address: {{.Gateway}}
        next-hop-interface: eno1
        table-id: 254
`
	t, err := template.New("ac").Parse(tpl)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return t.Execute(f, map[string]string{
		"Name":     cfg.Metadata.Name,
		"Hostname": cfg.Node.Hostname,
		"MAC":      mac,
		"IP":       cfg.Node.IP,
		"DNS":      cfg.Network.DNS,
		"Gateway":  cfg.Network.Gateway,
	})
}

func createAgentImage(ctx context.Context, dir string, opts Options) error {
	args := []string{"agent", "create", "image", "--dir", dir, "--log-level=info"}
	line := "openshift-install " + strings.Join(args, " ")
	if opts.DryRun || opts.Manual {
		fmt.Fprintf(os.Stdout, "[manual] %s\n", line)
		return nil
	}
	if _, err := exec.LookPath("openshift-install"); err != nil {
		fmt.Fprintf(os.Stderr, "openshift-install not on PATH — skipping image create\n")
		fmt.Fprintf(os.Stdout, "run: %s\n", line)
		return nil
	}
	cmd := exec.CommandContext(ctx, "openshift-install", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func waitInstall(ctx context.Context, dir string) error {
	steps := [][]string{
		{"agent", "wait-for", "bootstrap-complete", "--dir", dir, "--log-level=info"},
		{"agent", "wait-for", "install-complete", "--dir", dir, "--log-level=info"},
	}
	for _, args := range steps {
		cmd := exec.CommandContext(ctx, "openshift-install", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("openshift-install %v: %w", args, err)
		}
	}
	return nil
}

func printManualHubNext(cfg *config.HubConfig, iso string, opts Options) {
	fmt.Println()
	fmt.Println("=== hub create: next steps ===")
	fmt.Printf("1. Ensure agent ISO exists: %s\n", iso)
	fmt.Printf("2. Attach ISO to VM %s and boot\n", cfg.Node.Hostname)
	fmt.Printf("3. openshift-install agent wait-for bootstrap-complete --dir %s\n", cfg.Hub.WorkDir)
	fmt.Printf("4. openshift-install agent wait-for install-complete --dir %s\n", cfg.Hub.WorkDir)
	fmt.Printf("5. export KUBECONFIG=%s/auth/kubeconfig\n", cfg.Hub.WorkDir)
	fmt.Println("6. Install MCE + ACM (mini-mock hub install-acm --config ...)")
	if opts.Manual {
		fmt.Println("(manual mode: provider commands printed above; no wait)")
	}
}

func readSecret(path, fileEnv, inlineEnv string) (string, error) {
	if path == "" {
		path = os.Getenv(fileEnv)
	}
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
	if v := os.Getenv(inlineEnv); v != "" {
		return strings.TrimSpace(v), nil
	}
	return "", fmt.Errorf("set --pull-secret / $%s / $%s", fileEnv, inlineEnv)
}

func readSSH(path string) (string, error) {
	if path == "" {
		path = os.Getenv("SSH_PUBLIC_KEY_FILE")
	}
	if path == "" {
		home, _ := os.UserHomeDir()
		for _, c := range []string{
			filepath.Join(home, ".ssh", "id_ed25519.pub"),
			filepath.Join(home, ".ssh", "id_rsa.pub"),
		} {
			if fileExists(c) {
				path = c
				break
			}
		}
	}
	if path == "" {
		if v := os.Getenv("SSH_PUBLIC_KEY"); v != "" {
			return strings.TrimSpace(v), nil
		}
		return "", fmt.Errorf("set --ssh-key / $SSH_PUBLIC_KEY_FILE / $SSH_PUBLIC_KEY")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func sanitizePullSecret(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), "'", `'\''`)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
