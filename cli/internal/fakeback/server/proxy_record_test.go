package server_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/recorder"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/routes"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/server"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/store"
)

// fakeUpstream returns an httptest.Server backed by a router that
// serves a couple of canned shapes. Stands in for the skill author's
// real backend during the proxy + record tests.
func fakeUpstream(t *testing.T) (*httptest.Server, *upstreamRecorder) {
	t.Helper()
	rec := &upstreamRecorder{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/boards", func(w http.ResponseWriter, r *http.Request) {
		rec.observe(r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"boards":[{"id":"b1","title":"Today"},{"id":"b2","title":"Backlog"}]}`))
	})
	mux.HandleFunc("/api/users/me", func(w http.ResponseWriter, r *http.Request) {
		rec.observe(r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"u1","email":"alice@example.com"}`))
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, rec
}

type upstreamRecorder struct {
	mu       sync.Mutex
	requests []*http.Request
}

func (u *upstreamRecorder) observe(r *http.Request) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.requests = append(u.requests, r.Clone(r.Context()))
}

func (u *upstreamRecorder) count() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return len(u.requests)
}

func newFakebackWithProxy(t *testing.T, db, upstream, recordTo string) *httptest.Server {
	t.Helper()
	dir := t.TempDir()
	if db != "" {
		if err := os.WriteFile(filepath.Join(dir, "db.json"), []byte(db), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	st, err := store.Open(store.Config{
		File:       filepath.Join(dir, "db.json"),
		IDStrategy: "int",
	}, logger)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	opts := server.Options{
		Logger:   logger,
		Store:    st,
		Upstream: upstream,
	}
	if recordTo != "" {
		rec := recorder.New(recordTo, logger)
		t.Cleanup(func() { _ = rec.Close() })
		opts.Record = rec.Func()
	}
	h, err := server.New(opts)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts
}

func TestProxyFallback_ForwardsOn404(t *testing.T) {
	upstream, upRec := fakeUpstream(t)
	ts := newFakebackWithProxy(t, `{"boards":[]}`, upstream.URL, "")

	// /api/boards isn't a fakeback resource — falls through to upstream.
	resp, err := http.Get(ts.URL + "/api/boards")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Today") {
		t.Errorf("upstream body not surfaced: %s", body)
	}
	if upRec.count() != 1 {
		t.Errorf("upstream got %d requests, want 1", upRec.count())
	}
}

func TestProxyFallback_LocalWinsOverUpstream(t *testing.T) {
	upstream, upRec := fakeUpstream(t)
	// boards collection exists locally with one entry. Upstream
	// should NOT be hit.
	ts := newFakebackWithProxy(t, `{"boards":[{"id":"local-1","title":"Local"}]}`, upstream.URL, "")

	resp, err := http.Get(ts.URL + "/boards/local-1")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Local") {
		t.Errorf("local body not surfaced: %s", body)
	}
	if upRec.count() != 0 {
		t.Errorf("upstream was hit %d time(s); local should have won", upRec.count())
	}
}

func TestProxyFallback_StampsFakeBackendHeaderOnProxiedResponse(t *testing.T) {
	upstream, _ := fakeUpstream(t)
	ts := newFakebackWithProxy(t, `{"boards":[]}`, upstream.URL, "")

	resp, err := http.Get(ts.URL + "/api/users/me")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	// The fakeback's selfid middleware runs before the proxy fallback
	// in chi's chain — every response carries the header, including
	// proxied ones. This is the contract that lets the sandbox
	// fetch-refusal hook fire even against a proxied response.
	if got := resp.Header.Get("X-Tavora-Fake-Backend"); got != "true" {
		t.Errorf("X-Tavora-Fake-Backend = %q on proxied response, want true", got)
	}
}

func TestRecorder_CapturesProxiedResponseToFile(t *testing.T) {
	upstream, _ := fakeUpstream(t)
	dir := t.TempDir()
	recordPath := filepath.Join(dir, "recorded.json")
	ts := newFakebackWithProxy(t, `{"boards":[]}`, upstream.URL, recordPath)

	// Trigger two distinct upstream hits.
	for _, p := range []string{"/api/boards", "/api/users/me"} {
		resp, err := http.Get(ts.URL + p)
		if err != nil {
			t.Fatalf("GET %s: %v", p, err)
		}
		_ = resp.Body.Close()
	}

	// Wait past the recorder's debounce flush.
	deadline := time.Now().Add(2 * time.Second)
	var raw []byte
	for time.Now().Before(deadline) {
		r, err := os.ReadFile(recordPath)
		if err == nil && len(r) > 0 {
			raw = r
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if len(raw) == 0 {
		t.Fatal("recorded.json never appeared on disk")
	}
	if !strings.Contains(string(raw), "/api/boards") {
		t.Errorf("recorded.json missing /api/boards: %s", raw)
	}
	if !strings.Contains(string(raw), "/api/users/me") {
		t.Errorf("recorded.json missing /api/users/me: %s", raw)
	}
	if !strings.Contains(string(raw), "Today") {
		t.Errorf("recorded.json missing canned body: %s", raw)
	}
}

func TestRecorder_ReplayLoopWithoutUpstream(t *testing.T) {
	// Phase 1: record against a live upstream.
	upstream, upRec := fakeUpstream(t)
	dir := t.TempDir()
	recordPath := filepath.Join(dir, "recorded.json")

	if err := os.WriteFile(filepath.Join(dir, "db.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	st, err := store.Open(store.Config{File: filepath.Join(dir, "db.json"), IDStrategy: "int"}, logger)
	if err != nil {
		t.Fatal(err)
	}
	rec := recorder.New(recordPath, logger)
	h, err := server.New(server.Options{
		Logger: logger, Store: st,
		Upstream: upstream.URL,
		Record:   rec.Func(),
	})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(h)

	resp, _ := http.Get(ts.URL + "/api/users/me")
	resp.Body.Close()
	if upRec.count() != 1 {
		t.Fatalf("phase 1 upstream count = %d, want 1", upRec.count())
	}

	// Force a flush, then tear down phase 1.
	if err := rec.Close(); err != nil {
		t.Fatalf("recorder Close: %v", err)
	}
	ts.Close()
	st.Close()

	// Phase 2: boot WITHOUT upstream. recorded.json auto-loaded as
	// custom routes via LoadFolderRoutes.
	merged, err := routes.LoadFolderRoutes(dir)
	if err != nil {
		t.Fatalf("LoadFolderRoutes: %v", err)
	}
	if len(merged) == 0 {
		t.Fatalf("LoadFolderRoutes returned no routes — phase 2 has nothing to replay")
	}

	st2, _ := store.Open(store.Config{File: filepath.Join(dir, "db.json"), IDStrategy: "int"}, logger)
	defer st2.Close()
	h2, _ := server.New(server.Options{
		Logger: logger, Store: st2,
		Routes: merged,
		// No Upstream → proxy fallback disabled. Replay is local-only.
	})
	ts2 := httptest.NewServer(h2)
	defer ts2.Close()

	resp2, err := http.Get(ts2.URL + "/api/users/me")
	if err != nil {
		t.Fatalf("phase 2 GET: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("phase 2 status = %d, want 200", resp2.StatusCode)
	}
	// Content-Length must NOT be the upstream's leaked length —
	// otherwise the replay body's framing mismatches and the client
	// sees "transfer closed with N bytes remaining". The recorder
	// filters volatile headers; verify the body still parses cleanly
	// to its full size.
	body, err := io.ReadAll(resp2.Body)
	if err != nil {
		t.Fatalf("read phase 2 body: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("phase 2 body did not parse: %v\nbytes=%q", err, body)
	}
	if got["email"] != "alice@example.com" {
		t.Errorf("phase 2 replay did not return captured body: %v", got)
	}
	// Upstream count must NOT have increased — phase 2 served from disk.
	if upRec.count() != 1 {
		t.Errorf("upstream count = %d after phase 2, want 1 (no new upstream hits)", upRec.count())
	}
}

func TestRoutesLoadFolderRoutes_RoutesYamlWinsOverRecorded(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "routes.yaml"), []byte(`
routes:
  - method: GET
    path: /api/users/me
    status: 200
    body:
      id: "curated"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "recorded.json"), []byte(`{
  "routes": [
    {
      "method": "GET",
      "path": "/api/users/me",
      "status": 200,
      "body": { "id": "captured" }
    },
    {
      "method": "GET",
      "path": "/api/extra",
      "status": 200,
      "body": { "ok": true }
    }
  ]
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	merged, err := routes.LoadFolderRoutes(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(merged) != 2 {
		t.Fatalf("merged has %d routes, want 2", len(merged))
	}
	// First entry must be the curated one.
	if merged[0].Path != "/api/users/me" {
		t.Errorf("merged[0] path = %q, want /api/users/me", merged[0].Path)
	}
	body, _ := merged[0].Body.(map[string]any)
	if body["id"] != "curated" {
		t.Errorf("/api/users/me body id = %v, want curated (routes.yaml should win)", body["id"])
	}
}
