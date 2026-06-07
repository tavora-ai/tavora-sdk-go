package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStartMockBackendsSpawnsOnePerFolder(t *testing.T) {
	dir := t.TempDir()
	for name, db := range map[string]string{
		"tasks-backend":    `{"boards":[{"id":"b1","title":"x"}]}`,
		"calendar-backend": `{"events":[{"id":"e1","title":"standup"}]}`,
	} {
		folder := filepath.Join(dir, "mocks", name)
		if err := os.MkdirAll(folder, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(folder, "db.json"), []byte(db), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mocks, shutdown, err := startMockBackends(context.Background(), dir, logger)
	if err != nil {
		t.Fatalf("startMockBackends: %v", err)
	}
	defer shutdown()
	if len(mocks) != 2 {
		t.Fatalf("got %d mocks, want 2", len(mocks))
	}

	// Sorted by folder name → calendar-backend first.
	if mocks[0].Name != "calendar-backend" {
		t.Errorf("mocks[0].Name = %q, want calendar-backend", mocks[0].Name)
	}
	if mocks[1].Name != "tasks-backend" {
		t.Errorf("mocks[1].Name = %q, want tasks-backend", mocks[1].Name)
	}

	// Give listeners a moment to bind.
	deadline := time.Now().Add(2 * time.Second)
	var resp *http.Response
	for time.Now().Before(deadline) {
		r, err := http.Get(mocks[0].URL + "/events")
		if err == nil {
			resp = r
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if resp == nil {
		t.Fatal("calendar-backend never responded")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("X-Tavora-Fake-Backend"); got != "true" {
		t.Errorf("X-Tavora-Fake-Backend = %q, want true", got)
	}
}

func TestStartMockBackendsNoMocksFolderIsNoop(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mocks, shutdown, err := startMockBackends(context.Background(), dir, logger)
	if err != nil {
		t.Fatalf("startMockBackends: %v", err)
	}
	defer shutdown()
	if len(mocks) != 0 {
		t.Errorf("got %d mocks, want 0 for a project without mocks/", len(mocks))
	}
}

func TestStartMockBackendsShutdownStopsListeners(t *testing.T) {
	dir := t.TempDir()
	folder := filepath.Join(dir, "mocks", "foo")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "db.json"), []byte(`{"items":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mocks, shutdown, err := startMockBackends(context.Background(), dir, logger)
	if err != nil {
		t.Fatal(err)
	}
	if len(mocks) != 1 {
		t.Fatal("expected 1 mock")
	}

	// Wait for listener to bind.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := http.Get(mocks[0].URL + "/items"); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	shutdown()

	// After shutdown, a request should fail to connect.
	client := &http.Client{Timeout: 1 * time.Second}
	if _, err := client.Get(mocks[0].URL + "/items"); err == nil {
		t.Error("mock still responding after shutdown")
	}
}
