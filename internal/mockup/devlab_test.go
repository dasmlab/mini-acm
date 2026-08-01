package mockup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSeedDevLabGapsClearsPlaceholders(t *testing.T) {
	root := t.TempDir()
	store, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	m, err := store.Create(CreateReq{
		Name: "lab-rack-dev", Style: StyleACMMultiCluster, SeedDevLab: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if isPlaceholderPath(m.Spec.Gaps.PullSecretFile) || isPlaceholderPath(m.Spec.Gaps.SSHPublicKeyFile) {
		t.Fatalf("gaps still placeholders: pull=%q ssh=%q", m.Spec.Gaps.PullSecretFile, m.Spec.Gaps.SSHPublicKeyFile)
	}
	if !strings.Contains(m.Spec.Gaps.PullSecretFile, "dev-lab") {
		t.Fatalf("pull secret not under dev-lab: %s", m.Spec.Gaps.PullSecretFile)
	}
	for _, c := range m.Spec.Clusters {
		if isPlaceholderPath(c.DiscoveryISO) {
			t.Fatalf("cluster %s still placeholder ISO", c.Name)
		}
		if _, err := os.Stat(c.DiscoveryISO); err != nil {
			t.Fatalf("iso missing: %v", err)
		}
	}
	pub := m.Spec.Gaps.SSHPublicKeyFile
	b, err := os.ReadFile(pub)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(b), "ssh-ed25519 ") {
		t.Fatalf("bad pub key: %s", b)
	}
	priv := filepath.Join(filepath.Dir(pub), "id_ed25519")
	if _, err := os.Stat(priv); err != nil {
		t.Fatal(err)
	}

	res, _, err := store.ValidatePlan(m.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("validate not OK: %+v", res)
	}
	for _, i := range res.Issues {
		if i.Severity == "warn" && (i.Code == "gap-pull-secret" || i.Code == "gap-ssh-key" || i.Code == "gap-discovery-iso") {
			t.Fatalf("unexpected gap warning after seed: %+v", i)
		}
	}
}
