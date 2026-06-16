package store_test

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/store"
)

func TestReloadPicksUpFileChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "db.json")
	if err := os.WriteFile(path, []byte(`{"boards":[{"id":"b1","title":"Today"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	s, err := store.Open(store.Config{File: path, IDStrategy: "int"}, logger)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	rec, err := s.Get("boards", "b1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec["title"] != "Today" {
		t.Errorf("initial title = %v, want Today", rec["title"])
	}

	// Edit the file on disk (mimicking the skill author editing
	// db.json) and reload.
	if err := os.WriteFile(path, []byte(`{"boards":[{"id":"b1","title":"Tomorrow"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	rec2, _ := s.Get("boards", "b1")
	if rec2["title"] != "Tomorrow" {
		t.Errorf("after reload title = %v, want Tomorrow", rec2["title"])
	}
}

func TestResetAfterReloadReturnsToReloadedState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "db.json")
	if err := os.WriteFile(path, []byte(`{"boards":[{"id":"b1","title":"v1"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	s, err := store.Open(store.Config{File: path, IDStrategy: "int"}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Mutate in-memory.
	_, _ = s.Patch("boards", "b1", map[string]any{"title": "in-mem"})

	// Edit the file and Reload — seed should refresh to the new on-disk state.
	if err := os.WriteFile(path, []byte(`{"boards":[{"id":"b1","title":"v2-on-disk"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Reload(); err != nil {
		t.Fatal(err)
	}

	// Mutate again, then Reset — should now restore to v2 not v1.
	_, _ = s.Patch("boards", "b1", map[string]any{"title": "in-mem-2"})
	if err := s.Reset(); err != nil {
		t.Fatal(err)
	}
	rec, _ := s.Get("boards", "b1")
	if rec["title"] != "v2-on-disk" {
		t.Errorf("after reset title = %v, want v2-on-disk", rec["title"])
	}
}
