// Package libvirt implements the provider.Provider interface with virsh/virt-install.
package libvirt

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/dasmlab/mock-me/internal/provider"
)

func init() {
	provider.Register("libvirt", func(opts provider.Options) (provider.Provider, error) {
		return New(opts), nil
	})
}

// Libvirt drives local KVM via virsh / virt-install.
type Libvirt struct {
	opts provider.Options
	run  runner
}

type runner func(ctx context.Context, name string, args ...string) (string, error)

// New returns a libvirt provider.
func New(opts provider.Options) *Libvirt {
	return &Libvirt{opts: opts, run: defaultRunner}
}

func defaultRunner(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (l *Libvirt) Name() string { return "libvirt" }

func (l *Libvirt) exec(ctx context.Context, bin string, args ...string) error {
	line := bin + " " + strings.Join(args, " ")
	if l.opts.DryRun || l.opts.Manual {
		fmt.Fprintf(os.Stdout, "[%s] %s\n", dryOrManual(l.opts), line)
		return nil
	}
	out, err := l.run(ctx, bin, args...)
	if err != nil {
		return fmt.Errorf("%s: %w\n%s", line, err, out)
	}
	return nil
}

func dryOrManual(o provider.Options) string {
	if o.DryRun {
		return "dry-run"
	}
	return "manual"
}

func (l *Libvirt) EnsureNetwork(ctx context.Context, net provider.NetworkSpec) error {
	if net.Name == "" {
		return fmt.Errorf("network name required")
	}
	if net.Forward == "" {
		net.Forward = "nat"
	}
	xml := fmt.Sprintf(`<network>
  <name>%s</name>
  <forward mode='%s'/>
  <bridge name='%s' stp='on' delay='0'/>
  <ip address='%s' netmask='%s'>
    <dhcp>
      <range start='%s' end='%s'/>
    </dhcp>
  </ip>
</network>`,
		net.Name, net.Forward, bridgeName(net),
		net.Gateway, cidrToNetmask(net.CIDR),
		net.DHCPStart, net.DHCPEnd,
	)

	if l.opts.DryRun || l.opts.Manual {
		fmt.Fprintf(os.Stdout, "[%s] define libvirt network %s (gateway %s cidr %s)\n",
			dryOrManual(l.opts), net.Name, net.Gateway, net.CIDR)
		fmt.Fprintln(os.Stdout, xml)
		return nil
	}

	// Ignore define errors if network already exists; always try start.
	tmp, err := os.CreateTemp("", "mock-me-net-*.xml")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(xml); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	_, _ = l.run(ctx, "virsh", "net-define", tmp.Name())
	if err := l.exec(ctx, "virsh", "net-autostart", net.Name); err != nil {
		return err
	}
	return l.exec(ctx, "virsh", "net-start", net.Name)
}

func (l *Libvirt) CreateNode(ctx context.Context, node provider.NodeSpec) error {
	if node.Name == "" {
		return fmt.Errorf("node name required")
	}
	args := []string{
		"--name", node.Name,
		"--vcpus", fmt.Sprintf("%d", node.CPU),
		"--memory", fmt.Sprintf("%d", node.MemoryMiB),
		"--disk", fmt.Sprintf("size=%d,pool=%s,bus=virtio,format=qcow2", node.DiskGiB, orDefault(node.Pool, "default")),
		"--network", fmt.Sprintf("network=%s,mac=%s,model=virtio", orDefault(node.Network, "ocp-lab"), node.MAC),
		"--os-variant", "rhel9-unknown",
		"--noautoconsole",
		"--import",
		"--boot", "hd,cdrom",
	}
	if node.ISOPath != "" {
		args = append(args, "--disk", fmt.Sprintf("device=cdrom,path=%s", node.ISOPath))
	} else {
		// Create powered-off shell; ISO attached later.
		args = append(args, "--print-xml")
		// Prefer virt-install --cloud-init style empty disk + later ISO.
		// For MVP: virt-install with cdrom placeholder via --cdrom /dev/null is awkward;
		// create with --pxe then destroy netboot, or use virt-install --import after qemu-img.
		args = []string{
			"--name", node.Name,
			"--vcpus", fmt.Sprintf("%d", node.CPU),
			"--memory", fmt.Sprintf("%d", node.MemoryMiB),
			"--disk", fmt.Sprintf("size=%d,pool=%s,bus=virtio,format=qcow2", node.DiskGiB, orDefault(node.Pool, "default")),
			"--network", fmt.Sprintf("network=%s,mac=%s,model=virtio", orDefault(node.Network, "ocp-lab"), node.MAC),
			"--os-variant", "rhel9-unknown",
			"--noautoconsole",
			"--boot", "hd,cdrom",
			"--cdrom", "/dev/null",
			"--noreboot",
		}
	}

	line := "virt-install " + strings.Join(args, " ")
	if l.opts.DryRun || l.opts.Manual {
		fmt.Fprintf(os.Stdout, "[%s] %s\n", dryOrManual(l.opts), line)
		return nil
	}
	out, err := l.run(ctx, "virt-install", args...)
	if err != nil {
		// Domain may already exist.
		if strings.Contains(out, "already exists") {
			return nil
		}
		return fmt.Errorf("virt-install: %w\n%s", err, out)
	}
	return nil
}

func (l *Libvirt) DeleteNode(ctx context.Context, name string) error {
	_ = l.exec(ctx, "virsh", "destroy", name)
	return l.exec(ctx, "virsh", "undefine", name, "--remove-all-storage", "--nvram")
}

func (l *Libvirt) AttachISO(ctx context.Context, name, isoPath string) error {
	if isoPath == "" {
		return fmt.Errorf("iso path required")
	}
	// Change CDROM media and set boot order to cdrom.
	if err := l.exec(ctx, "virsh", "change-media", name, "sda", isoPath, "--insert", "--live", "--config"); err != nil {
		// Fallback device name hdc / sdb common on virt-install layouts.
		_ = l.exec(ctx, "virsh", "change-media", name, "hdc", isoPath, "--insert", "--config")
	}
	return l.exec(ctx, "virsh", "qemu-monitor-command", name, "--hmp", "change ide1-cd0 "+isoPath)
}

func (l *Libvirt) DetachISO(ctx context.Context, name string) error {
	_ = l.exec(ctx, "virsh", "change-media", name, "sda", "--eject", "--config")
	return l.exec(ctx, "virsh", "change-media", name, "hdc", "--eject", "--config")
}

func (l *Libvirt) StartNode(ctx context.Context, name string) error {
	return l.exec(ctx, "virsh", "start", name)
}

func (l *Libvirt) StopNode(ctx context.Context, name string) error {
	return l.exec(ctx, "virsh", "destroy", name)
}

func (l *Libvirt) GetMAC(ctx context.Context, name string) (string, error) {
	if l.opts.DryRun || l.opts.Manual {
		return "", nil
	}
	out, err := l.run(ctx, "virsh", "domiflist", name)
	if err != nil {
		return "", err
	}
	// Parse second column of first interface line after header.
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 5 && strings.Contains(fields[4], ":") {
			return fields[4], nil
		}
	}
	return "", fmt.Errorf("mac not found for %s", name)
}

func (l *Libvirt) GetPowerState(ctx context.Context, name string) (provider.PowerState, error) {
	if l.opts.DryRun || l.opts.Manual {
		return provider.PowerUnknown, nil
	}
	out, err := l.run(ctx, "virsh", "domstate", name)
	if err != nil {
		return provider.PowerUnknown, err
	}
	s := strings.TrimSpace(out)
	switch {
	case strings.Contains(s, "running"):
		return provider.PowerOn, nil
	case strings.Contains(s, "shut"):
		return provider.PowerOff, nil
	default:
		return provider.PowerUnknown, nil
	}
}

func (l *Libvirt) ListNodes(ctx context.Context, prefix string) ([]string, error) {
	if l.opts.DryRun || l.opts.Manual {
		return nil, nil
	}
	out, err := l.run(ctx, "virsh", "list", "--all", "--name")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, line := range strings.Split(out, "\n") {
		n := strings.TrimSpace(line)
		if n == "" {
			continue
		}
		if prefix == "" || strings.HasPrefix(n, prefix) {
			names = append(names, n)
		}
	}
	return names, nil
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func bridgeName(net provider.NetworkSpec) string {
	if net.Bridge != "" {
		return net.Bridge
	}
	// libvirt truncates; keep short.
	n := net.Name
	if len(n) > 10 {
		n = n[:10]
	}
	return "virbr-" + n
}

// cidrToNetmask is a tiny helper for /24 labs (MVP). Extend later.
func cidrToNetmask(cidr string) string {
	if strings.HasSuffix(cidr, "/24") {
		return "255.255.255.0"
	}
	if strings.HasSuffix(cidr, "/16") {
		return "255.255.0.0"
	}
	return "255.255.255.0"
}
