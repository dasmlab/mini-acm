package mockup

import "testing"

func TestCatalogHasMiniACM(t *testing.T) {
	c := Catalog()
	if len(c.Genres) < 2 {
		t.Fatalf("want genres, got %d", len(c.Genres))
	}
	s := LookupStyle(StyleMiniACMMultiCluster)
	if s == nil || !s.Available {
		t.Fatal("mini-acm style must be available")
	}
	sno := LookupStyle(StyleSingleSNOOCP)
	if sno == nil || !sno.Available {
		t.Fatal("single-sno-ocp style must be available")
	}
	if len(s.Relations) < 3 {
		t.Fatalf("want relation rules, got %d", len(s.Relations))
	}
	win := LookupStyle(StyleWindowsUI)
	if win == nil || win.Available {
		t.Fatal("windows-ui should exist as unavailable stub")
	}
}

func TestResolveCreateStyle(t *testing.T) {
	g, st, def, err := ResolveCreateStyle("", "")
	if err != nil || g != GenreClusterManagement || st != StyleMiniACMMultiCluster || def == nil {
		t.Fatalf("defaults: g=%s st=%s err=%v", g, st, err)
	}
	_, _, _, err = ResolveCreateStyle(GenreApplicationDevelopment, StyleWindowsUI)
	if err == nil {
		t.Fatal("expected stub style rejected")
	}
	_, _, _, err = ResolveCreateStyle(GenreApplicationDevelopment, StyleMiniACMMultiCluster)
	if err == nil {
		t.Fatal("expected genre mismatch")
	}
}

func TestCreateSetsGenreStyle(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.Create(CreateReq{Name: "rack1"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Spec.Genre != GenreClusterManagement || m.Spec.Style != StyleMiniACMMultiCluster {
		t.Fatalf("genre/style: %s / %s", m.Spec.Genre, m.Spec.Style)
	}
}

func TestCreateSingleSNO(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.Create(CreateReq{
		Name: "sno1", Genre: GenreClusterManagement, Style: StyleSingleSNOOCP,
	})
	if err != nil {
		t.Fatal(err)
	}
	if m.Spec.Style != StyleSingleSNOOCP || m.Spec.Hub.Label != "OCP-MGMT" {
		t.Fatalf("sno: style=%s hub=%s", m.Spec.Style, m.Spec.Hub.Label)
	}
	if m.Spec.ACM.Enabled || m.Spec.Hub.InstallACM {
		t.Fatal("SNO style should not enable ACM")
	}
	if len(m.Spec.Clusters) != 0 {
		t.Fatalf("want 0 deployments, got %d", len(m.Spec.Clusters))
	}
	res := ValidateTopology(m)
	if !res.OK {
		t.Fatalf("SNO validate: %+v", res)
	}
}
