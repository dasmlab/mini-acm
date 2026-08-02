package deploy

import (
	"strings"
	"testing"
)

func TestRemoteWorkRoot(t *testing.T) {
	if got := remoteWorkRoot("lab-rack-1"); got != "/vm-disks/mock-me/work/lab-rack-1" {
		t.Fatalf("got %q", got)
	}
}

func TestEnsureHostLayoutScript(t *testing.T) {
	s := ensureHostLayoutScript(HostPoolName)
	for _, want := range []string{"/vm-disks/mock-me", "/vm-disks/mock-me/images", `POOL="mock-me"`, "HOST_ROOT="} {
		if !strings.Contains(s, want) {
			t.Errorf("layout script missing %q", want)
		}
	}
}
