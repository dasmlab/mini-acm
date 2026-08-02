package deploy

import (
	"fmt"
	"strings"

	"github.com/dasmlab/mock-me/internal/mockup"
)

// guestSpec is one libvirt domain to materialize on the inventory MACHINE-HOST.
type guestSpec struct {
	Name      string
	Role      string // gw | hub | spoke
	CPU       int
	MemoryMiB int
	DiskGiB   int
	MAC       string
	IP        string
}

func buildGuests(m *mockup.MockUp) []guestSpec {
	netName := m.Spec.InfraHost.NetworkName
	if netName == "" {
		netName = "ocp-lab"
	}
	_ = netName

	var out []guestSpec
	gw := m.Spec.Gateway
	if gw.ID != "" {
		name := gw.Hostname
		if name == "" {
			name = "vyos-lab-gw"
		}
		cpu, mem, disk := gw.CPU, gw.MemoryMiB, gw.DiskGiB
		if cpu == 0 {
			cpu = 2
		}
		if mem == 0 {
			mem = 2048
		}
		if disk == 0 {
			disk = 10
		}
		out = append(out, guestSpec{
			Name: name, Role: "gw", CPU: cpu, MemoryMiB: mem, DiskGiB: disk,
			MAC: "52:54:00:13:00:10", IP: gw.LANIP,
		})
	}

	h := m.Spec.Hub
	if h.ID != "" {
		name := h.Hostname
		if name == "" {
			name = "hub-sno"
		}
		cpu, mem, disk := h.CPU, h.MemoryMiB, h.DiskGiB
		if cpu == 0 {
			cpu = 8
		}
		if mem == 0 {
			mem = 24576
		}
		if disk == 0 {
			disk = 200
		}
		mac := h.MAC
		if mac == "" {
			mac = "52:54:00:13:00:20"
		}
		out = append(out, guestSpec{
			Name: name, Role: "hub", CPU: cpu, MemoryMiB: mem, DiskGiB: disk,
			MAC: mac, IP: h.IP,
		})
	}

	if m.Spec.Style == mockup.StyleSingleSNOOCP {
		return out
	}
	for _, c := range m.Spec.Clusters {
		n := c.Count
		if n <= 0 {
			n = 3
		}
		cpu, mem, disk := c.CPU, c.MemoryMiB, c.DiskGiB
		if cpu == 0 {
			cpu = 4
		}
		if mem == 0 {
			mem = 16384
		}
		if disk == 0 {
			disk = 120
		}
		prefix := c.MACPrefix
		if prefix == "" {
			prefix = "52:54:00:20:00"
		}
		baseName := c.Name
		if baseName == "" {
			baseName = c.ID
		}
		for i := 0; i < n; i++ {
			mac := fmt.Sprintf("%s:%02x", strings.TrimRight(prefix, ":"), i+1)
			out = append(out, guestSpec{
				Name: fmt.Sprintf("%s-%d", baseName, i), Role: "spoke",
				CPU: cpu, MemoryMiB: mem, DiskGiB: disk, MAC: mac,
			})
		}
	}
	return out
}

// ensureGuestsScript creates shut-off libvirt domains + qcow2 disks for the MockUp.
func ensureGuestsScript(m *mockup.MockUp, guests []guestSpec) string {
	pool := m.Spec.InfraHost.StoragePool
	if pool == "" {
		pool = "default"
	}
	net := m.Spec.InfraHost.NetworkName
	if net == "" {
		net = "ocp-lab"
	}

	var b strings.Builder
	b.WriteString("set -eu\n")
	b.WriteString("export LIBVIRT_DEFAULT_URI=\"${LIBVIRT_DEFAULT_URI:-qemu:///system}\"\n")
	b.WriteString("command -v virt-install >/dev/null\n")
	b.WriteString("command -v virsh >/dev/null\n")
	fmt.Fprintf(&b, "POOL=%q\nNET=%q\n", pool, net)
	b.WriteString(`POOL_PATH=$(virsh pool-dumpxml "$POOL" 2>/dev/null | sed -n 's/.*<path>\([^<]*\)<\/path>.*/\1/p' | head -1)
[ -n "$POOL_PATH" ] || POOL_PATH="$HOME/libvirt-images"
mkdir -p "$POOL_PATH"
echo "POOL_PATH=$POOL_PATH"
`)

	for _, g := range guests {
		fmt.Fprintf(&b, "\n# --- guest %s (%s) ---\n", g.Name, g.Role)
		fmt.Fprintf(&b, "NAME=%q\nCPU=%d\nMEM=%d\nDISK=%d\nMAC=%q\n",
			g.Name, g.CPU, g.MemoryMiB, g.DiskGiB, g.MAC)
		b.WriteString(`if virsh dominfo "$NAME" >/dev/null 2>&1; then
  echo "EXISTS $NAME ($(virsh domstate "$NAME" 2>/dev/null | tr -d '\n'))"
else
  echo "CREATE $NAME cpu=$CPU memMiB=$MEM diskGiB=$DISK mac=$MAC"
  if ! virsh vol-info --pool "$POOL" "${NAME}.qcow2" >/dev/null 2>&1; then
    virsh vol-create-as "$POOL" "${NAME}.qcow2" "${DISK}G" --format qcow2
  fi
  virt-install \
    --name "$NAME" \
    --vcpus "$CPU" \
    --memory "$MEM" \
    --disk "vol=${POOL}/${NAME}.qcow2,bus=virtio" \
    --network "network=${NET},mac=${MAC},model=virtio" \
    --os-variant rhel9-unknown \
    --boot hd,cdrom \
    --graphics none \
    --noautoconsole \
    --noreboot \
    --import \
    || { echo "virt-install failed for $NAME"; exit 1; }
  virsh destroy "$NAME" 2>/dev/null || true
  virsh pool-refresh "$POOL" 2>/dev/null || true
  echo "DEFINED $NAME vol=${POOL}/${NAME}.qcow2"
fi
`)
	}

	b.WriteString(`
echo "GUESTS_DEFINED=1"
echo "VIRSH_LIST<<"
virsh list --all || true
echo ">>VIRSH_LIST"
echo "POOL_VOLS<<"
virsh vol-list "$POOL" 2>/dev/null || ls -la "$POOL_PATH" || true
echo ">>POOL_VOLS"
`)
	return b.String()
}

// destroyGuestsScript undefines MockUp domains and removes their pool volumes + remote work dir.
func destroyGuestsScript(m *mockup.MockUp, guests []guestSpec, remoteWorkRoot string) string {
	pool := m.Spec.InfraHost.StoragePool
	if pool == "" {
		pool = "default"
	}
	var b strings.Builder
	b.WriteString("set -eu\n")
	b.WriteString("export LIBVIRT_DEFAULT_URI=\"${LIBVIRT_DEFAULT_URI:-qemu:///system}\"\n")
	fmt.Fprintf(&b, "POOL=%q\n", pool)
	fmt.Fprintf(&b, "WORK=%q\n", remoteWorkRoot)
	b.WriteString(`echo "TEARDOWN_START"
`)
	for _, g := range guests {
		fmt.Fprintf(&b, "NAME=%q\n", g.Name)
		b.WriteString(`if virsh dominfo "$NAME" >/dev/null 2>&1; then
  echo "DESTROY $NAME"
  virsh destroy "$NAME" 2>/dev/null || true
  virsh undefine "$NAME" --remove-all-storage 2>/dev/null \
    || virsh undefine "$NAME" 2>/dev/null \
    || true
fi
if virsh vol-info --pool "$POOL" "${NAME}.qcow2" >/dev/null 2>&1; then
  echo "VOL_DELETE ${NAME}.qcow2"
  virsh vol-delete --pool "$POOL" "${NAME}.qcow2" 2>/dev/null || true
fi
`)
	}
	b.WriteString(`if [ -n "$WORK" ] && [ -d "$WORK" ]; then
  echo "RM_WORK $WORK"
  rm -rf "$WORK"
fi
echo "VIRSH_AFTER<<"
virsh list --all || true
echo ">>VIRSH_AFTER"
echo TEARDOWN_OK=1
`)
	return b.String()
}
