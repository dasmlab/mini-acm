package deploy

import (
	"fmt"
	"strings"

	"github.com/dasmlab/mock-me/internal/eeimage"
)

func EEImage() string { return eeimage.Image() }

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func eeImageShell() string {
	return shellQuote(EEImage())
}

// eeEnsureScript pulls the curated EE if needed and verifies openshift-install + oc inside it.
func eeEnsureScript() string {
	img := eeImageShell()
	return fmt.Sprintf(`
EE_IMAGE=%s
echo "EE_IMAGE=$EE_IMAGE"
if ! command -v podman >/dev/null 2>&1; then
  echo "HAS_PODMAN=0"
  echo "EE_FAIL=podman"
  exit 0
fi
echo "HAS_PODMAN=1"
echo "podman=$(podman --version 2>/dev/null | head -1)"
if ! podman image exists "$EE_IMAGE" 2>/dev/null; then
  echo "PULLING $EE_IMAGE …"
  if ! podman pull "$EE_IMAGE"; then
    echo "EE_FAIL=ee-image"
    exit 0
  fi
fi
echo "EE_IMAGE_PRESENT=1"
INST_OUT=$(podman run --rm "$EE_IMAGE" version 2>&1 | sed -n '1,3p' || true)
echo "openshift-install<<"
echo "$INST_OUT"
echo ">>openshift-install"
if ! echo "$INST_OUT" | grep -qi openshift; then
  echo "EE_FAIL=ee-tools"
  exit 0
fi
if ! podman run --rm --entrypoint /usr/local/bin/oc "$EE_IMAGE" version --client >/dev/null 2>&1; then
  echo "EE_FAIL=ee-tools"
  exit 0
fi
echo "HAS_INSTALLER=1"
echo "EE_OK=1"
`, img)
}
