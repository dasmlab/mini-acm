package deploy

import (
	"fmt"
	"path"
	"strings"
)

// Host disk layout on MACHINE-HOST (large data volume — keep root/home small).
//
//	/vm-disks/mock-me/
//	  images/   libvirt storage pool "mock-me"
//	  work/<mockup>/   staged YAML, hub agent ISO inputs, ACM notes
const (
	HostDataRoot  = "/vm-disks/mock-me"
	HostImagesDir = HostDataRoot + "/images"
	HostWorkDir   = HostDataRoot + "/work"
	HostPoolName  = "mock-me"
)

func remoteWorkRoot(mockupName string) string {
	name := strings.TrimSpace(mockupName)
	if name == "" {
		name = "unnamed"
	}
	// path.Join cleans; keep absolute.
	return path.Join(HostWorkDir, name)
}

func ensureHostLayoutScript(poolName string) string {
	if poolName == "" {
		poolName = HostPoolName
	}
	return fmt.Sprintf(`set -eu
export LIBVIRT_DEFAULT_URI="${LIBVIRT_DEFAULT_URI:-qemu:///system}"
ROOT=%q
IMAGES=%q
POOL=%q
# Prefer the large /vm-disks volume; fall back only if missing.
if [ ! -d /vm-disks ]; then
  echo "WARN: /vm-disks missing — using $HOME/mock-me (root/home may fill)"
  ROOT="$HOME/mock-me"
  IMAGES="$ROOT/images"
fi
# Ensure dasm (or deploy SSH user) can write; qemu can read.
if [ ! -w /vm-disks ] 2>/dev/null; then
  sudo -n mkdir -p "$IMAGES" "$ROOT/work" 2>/dev/null || true
  sudo -n chown -R "$(id -u):$(id -g)" /vm-disks 2>/dev/null || true
  sudo -n chmod 755 /vm-disks 2>/dev/null || true
fi
mkdir -p "$IMAGES" "$ROOT/work" || {
  echo "ERROR: cannot create $ROOT — as root: chown -R $(id -un):$(id -gn) /vm-disks && chmod 755 /vm-disks"
  exit 1
}
chmod 755 /vm-disks 2>/dev/null || true
chmod 755 "$ROOT" "$IMAGES" "$ROOT/work" 2>/dev/null || true
chmod o+x /vm-disks "$ROOT" "$IMAGES" 2>/dev/null || true
if command -v setfacl >/dev/null 2>&1; then
  setfacl -m u:qemu:rwx /vm-disks "$ROOT" "$IMAGES" 2>/dev/null || true
fi
if command -v semanage >/dev/null 2>&1; then
  semanage fcontext -a -t virt_image_t "$IMAGES(/.*)?" 2>/dev/null || true
  restorecon -Rv "$IMAGES" 2>/dev/null || true
elif command -v chcon >/dev/null 2>&1; then
  chcon -Rt virt_image_t "$IMAGES" 2>/dev/null || true
fi
if ! virsh pool-info "$POOL" >/dev/null 2>&1; then
  virsh pool-define-as "$POOL" dir --target "$IMAGES"
  virsh pool-build "$POOL" || true
  virsh pool-start "$POOL" || true
  virsh pool-autostart "$POOL" || true
else
  # Re-point an existing mock-me pool if it still targets $HOME.
  CUR=$(virsh pool-dumpxml "$POOL" 2>/dev/null | sed -n 's/.*<path>\([^<]*\)<\/path>.*/\1/p' | head -1)
  if [ -n "$CUR" ] && [ "$CUR" != "$IMAGES" ]; then
    echo "POOL_REPOINT $POOL $CUR -> $IMAGES"
    virsh pool-destroy "$POOL" 2>/dev/null || true
    virsh pool-undefine "$POOL" 2>/dev/null || true
    virsh pool-define-as "$POOL" dir --target "$IMAGES"
    virsh pool-build "$POOL" || true
    virsh pool-start "$POOL" || true
    virsh pool-autostart "$POOL" || true
  else
    virsh pool-start "$POOL" 2>/dev/null || true
    virsh pool-autostart "$POOL" 2>/dev/null || true
  fi
fi
virsh pool-refresh "$POOL" 2>/dev/null || true
echo "HOST_ROOT=$ROOT"
echo "HOST_IMAGES=$IMAGES"
echo "HOST_POOL=$POOL"
echo "POOL_INFO<<"
virsh pool-info "$POOL" || true
echo ">>POOL_INFO"
`, HostDataRoot, HostImagesDir, poolName)
}
