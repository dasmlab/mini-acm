package activity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreAppendList(t *testing.T) {
	dir := t.TempDir()
	st, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := st.Append(Event{Type: TypeLogin, User: "alice"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.Append(Event{Type: TypeNavigate, User: "bob", Path: "/inventory", DwellMs: 1200, VisibleMs: 1000, EngagedMs: 400}); err != nil {
		t.Fatal(err)
	}
	evs, err := st.List(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 4 {
		t.Fatalf("want 4 events, got %d", len(evs))
	}
	if evs[0].Type != TypeNavigate || evs[0].User != "bob" {
		t.Fatalf("newest should be navigate/bob: %+v", evs[0])
	}
	path := filepath.Join(dir, "activity", "events.jsonl")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
