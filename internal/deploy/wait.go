package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func envDuration(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

func ocpWaitTimeout() time.Duration { return envDuration("MOCK_ME_OCP_WAIT", 90*time.Minute) }
func acmWaitTimeout() time.Duration { return envDuration("MOCK_ME_ACM_WAIT", 45*time.Minute) }

func localHubKubeconfigPath(dataRoot, mockupName string) string {
	name := strings.TrimSpace(mockupName)
	if name == "" {
		name = "hub"
	}
	return filepath.Join(dataRoot, fmt.Sprintf("hub-%s", name), "auth", "kubeconfig")
}

// waitForHubInstall polls until agent install-complete (preferred) or live API via oc.
func (e *Engine) waitForHubInstall(j *Job, remoteRoot, hubName, hubIP, apiHost, apiIntHost string) (string, error) {
	deadline := time.Now().Add(ocpWaitTimeout())
	img := EEImage()
	remoteKC := remoteRoot + "/hub/auth/kubeconfig"
	if hubIP == "" {
		hubIP = "10.77.30.20"
	}
	e.log(j, StageOCP, "waiting up to %s for agent install-complete / kubeconfig (hub=%s api=%s)", ocpWaitTimeout(), hubName, apiHost)
	e.log(j, StageOCP, "console: virsh --connect qemu:///system console %s", hubName)
	_ = e.SaveJob(j)

	var last string
	for attempt := 1; time.Now().Before(deadline); attempt++ {
		script := fmt.Sprintf(`set -eu
export LIBVIRT_DEFAULT_URI="${LIBVIRT_DEFAULT_URI:-qemu:///system}"
ROOT=%q
EE_IMAGE=%q
HUB=%q
HUB_IP=%q
API_HOST=%q
API_INT_HOST=%q
KC="$ROOT/hub/auth/kubeconfig"
STATE=$(virsh --connect qemu:///system domstate "$HUB" 2>/dev/null | tr -d '\n' || echo missing)
echo "HUB_STATE=$STATE"
echo "POLL_ATTEMPT=%d"
set +e
# Keep asset kubeconfig pristine for openshift-install (CA + insecure together is rejected).
# Resolve api/api-int via --add-host / libvirt DNS instead of rewriting the server URL.
if [ -f "$KC" ] && grep -q 'insecure-skip-tls-verify' "$KC"; then
  if [ -f "$KC.bak-lab" ]; then cp -a "$KC.bak-lab" "$KC"; fi
  if [ -f "$KC.bak" ]; then cp -a "$KC.bak" "$KC"; fi
  # Last resort: strip insecure line if bak missing.
  grep -v 'insecure-skip-tls-verify' "$KC" > "$KC.fix" 2>/dev/null && mv "$KC.fix" "$KC"
  echo "KUBECONFIG_RESTORED=1"
fi
# Prefer installer wait-for. No --timeout flag — bound with coreutils timeout.
WAIT_OUT=$(timeout 100s podman run --rm \
  --add-host "${API_HOST}:${HUB_IP}" \
  --add-host "${API_INT_HOST}:${HUB_IP}" \
  --dns 10.77.30.1 \
  -v "$ROOT/hub:/output:Z" \
  --entrypoint /usr/local/bin/openshift-install \
  "$EE_IMAGE" agent wait-for install-complete --dir /output 2>&1)
WAIT_RC=$?
echo "$WAIT_OUT" | tail -n 25
if [ "$WAIT_RC" -eq 0 ]; then
  echo "INSTALL_COMPLETE=1"
fi
# BIP reboot must land on HD: after image write creates partitions, flip boot and
# eject ISO *while still on live agent*. Waiting until root leaves squashfs is too
# late — cdrom boot re-runs --format-disk and loops the install.
# Note: virsh dumpxml (live) may still show old boot order; --inactive is authoritative.
if [ ! -f /tmp/mock-me-hub-boot-hd.done ] && [ -f "$ROOT/hub/id_install" ]; then
  PARTS=$(ssh -i "$ROOT/hub/id_install" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
    -o BatchMode=yes -o ConnectTimeout=5 "core@${HUB_IP}" \
    "lsblk -n -o TYPE | grep -c part || true" 2>/dev/null || echo 0)
  echo "DISK_PARTS=$PARTS"
  WRITE_PCT=$(echo "$WAIT_OUT" | sed -n 's/.*Writing image to disk: \([0-9][0-9]*\)%%.*/\1/p' | tail -n 1)
  echo "WRITE_PCT=${WRITE_PCT:-0}"
  if [ "${PARTS:-0}" -gt 0 ] 2>/dev/null || [ "${WRITE_PCT:-0}" -ge 90 ] 2>/dev/null; then
    python3 - <<'PY' || true
import subprocess, re
xml = subprocess.check_output(["virsh", "dumpxml", "--inactive", "hub-sno"], text=True)
m = re.search(r"<os>[\s\S]*?</os>", xml)
if not m:
    raise SystemExit(0)
os_old = m.group(0)
os_new = re.sub(r"<boot[^/]*/>\s*", "", os_old)
if "<type" in os_new:
    os_new = os_new.replace("</os>", "    <boot dev=\"hd\"/>\n    <boot dev=\"cdrom\"/>\n  </os>")
xml2 = xml.replace(os_old, os_new, 1)
xml2 = re.sub(r"<source file=\"[^\"]*hub-sno-agent\.iso\"/>\s*", "", xml2)
xml2 = re.sub(r"<source file='[^']*hub-sno-agent\.iso'/>\s*", "", xml2)
open("/tmp/mock-me-hub-boot-hd.xml", "w").write(xml2)
subprocess.check_call(["virsh", "define", "/tmp/mock-me-hub-boot-hd.xml"])
print("BOOT_XML_HD=1")
PY
    virsh change-media "$HUB" sda --eject --live --config 2>/dev/null || true
    virsh change-media "$HUB" hdc --eject --live --config 2>/dev/null || true
    virsh change-media "$HUB" hda --eject --live --config 2>/dev/null || true
    touch /tmp/mock-me-hub-boot-hd.done
    echo "BOOT_FLIPPED_HD=1"
  fi
fi
if [ -f "$KC" ]; then
  echo "KUBECONFIG_PRESENT=1"
  # Probe API with a disposable insecure copy (do not mutate asset kubeconfig).
  cp -a "$KC" /tmp/mock-me-oc-probe.kubeconfig
  sed -i -e "s|https://api\\.[^:]*:6443|https://${HUB_IP}:6443|" /tmp/mock-me-oc-probe.kubeconfig
  if ! grep -q 'insecure-skip-tls-verify' /tmp/mock-me-oc-probe.kubeconfig; then
    sed -i -e "s|server: https://${HUB_IP}:6443|insecure-skip-tls-verify: true\\n    server: https://${HUB_IP}:6443|" /tmp/mock-me-oc-probe.kubeconfig
  fi
  # Remove CA when using insecure — oc tolerates this better than install wait-for.
  grep -v 'certificate-authority' /tmp/mock-me-oc-probe.kubeconfig > /tmp/mock-me-oc-probe.kubeconfig.2 2>/dev/null \
    && mv /tmp/mock-me-oc-probe.kubeconfig.2 /tmp/mock-me-oc-probe.kubeconfig
  podman run --rm \
    --add-host "${API_HOST}:${HUB_IP}" \
    --add-host "${API_INT_HOST}:${HUB_IP}" \
    --dns 10.77.30.1 \
    -v /tmp/mock-me-oc-probe.kubeconfig:/auth/kubeconfig:Z \
    -e KUBECONFIG=/auth/kubeconfig \
    --entrypoint /usr/local/bin/oc \
    "$EE_IMAGE" get ns default >/dev/null 2>&1
  OC_RC=$?
  if [ "$OC_RC" -eq 0 ]; then
    echo "API_OK=1"
  else
    echo "API_OK=0"
  fi
else
  echo "KUBECONFIG_PRESENT=0"
fi
curl -k -s -o /dev/null -w "READYZ_IP=%%{http_code}\n" --connect-timeout 2 "https://${HUB_IP}:6443/readyz" || echo READYZ_IP=fail
set -e
`, remoteRoot, img, hubName, hubIP, apiHost, apiIntHost, attempt)

		out, _, err := e.inv.RunScript(j.InventoryID, script)
		if err != nil {
			last = truncate(out, 600)
			e.log(j, StageOCP, "poll #%d ssh error: %v (%s)", attempt, err, last)
			_ = e.SaveJob(j)
		} else {
			last = trimHostOut(out)
			e.log(j, StageOCP, "poll #%d hub_state / kubeconfig:\n%s", attempt, last)
			_ = e.SaveJob(j)
			// Prefer installer install-complete. API_OK alone is too early for ACM
			// (bootstrap API answers while clusterversion is still Progressing).
			if strings.Contains(out, "INSTALL_COMPLETE=1") {
				raw, rerr := e.inv.ReadRemoteFile(j.InventoryID, remoteKC)
				if rerr != nil {
					return "", fmt.Errorf("read remote kubeconfig: %w", rerr)
				}
				if len(raw) < 32 {
					return "", fmt.Errorf("remote kubeconfig empty at %s", remoteKC)
				}
				return string(raw), nil
			}
		}
		remain := time.Until(deadline).Round(time.Second)
		if remain <= 0 {
			break
		}
		sleep := 15 * time.Second
		if remain < sleep {
			sleep = remain
		}
		time.Sleep(sleep)
	}
	return "", fmt.Errorf("OCP-MGMT timed out after %s waiting for hub kubeconfig/API (last: %s)", ocpWaitTimeout(), last)
}

func (e *Engine) persistHubKubeconfig(j *Job, mockupName, contents string) (string, error) {
	path := localHubKubeconfigPath(e.mockups.DataRoot(), mockupName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		return "", err
	}
	m, err := e.mockups.Get(j.MockUpID)
	if err != nil {
		return path, err
	}
	m.Spec.Gaps.HubKubeconfig = path
	if err := e.mockups.Save(m); err != nil {
		return path, err
	}
	e.log(j, StageOCP, "persisted hub kubeconfig → %s", path)
	_ = e.SaveJob(j)
	return path, nil
}
