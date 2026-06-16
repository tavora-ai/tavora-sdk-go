package server_test

import (
	"bufio"
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

	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/requestlog"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/server"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/store"
)

// newFakebackWithRequestLog wires a requestlog.Logger into a stock
// fakeback server backed by the supplied db.json. Returns the test
// server and the open log file path so tests can read it after
// hitting endpoints.
func newFakebackWithRequestLog(t *testing.T, db string) (*httptest.Server, string) {
	t.Helper()
	dir := t.TempDir()
	if db != "" {
		if err := os.WriteFile(filepath.Join(dir, "db.json"), []byte(db), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	st, err := store.Open(store.Config{File: filepath.Join(dir, "db.json"), IDStrategy: "int"}, logger)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	logPath := filepath.Join(dir, "requests.jsonl")
	reqLog, err := requestlog.Open(requestlog.Options{Path: logPath}, logger)
	if err != nil {
		t.Fatalf("requestlog.Open: %v", err)
	}
	t.Cleanup(func() { _ = reqLog.Close() })

	h, err := server.New(server.Options{
		Logger:     logger,
		Store:      st,
		RequestLog: reqLog,
	})
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts, logPath
}

// readLogLines flushes the logger (if needed), reads the file, and
// returns one parsed entry per line. The logger writes synchronously,
// so no debounce-wait is needed.
func readLogLines(t *testing.T, path string) []map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	defer f.Close()
	var out []map[string]any
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e map[string]any
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("parse line: %v\n  line=%q", err, sc.Text())
		}
		out = append(out, e)
	}
	return out
}

const reqLogDB = `{
  "boards": [
    {"id":"b1","title":"Today"}
  ]
}`

func TestRequestLog_OneEntryPerRequest(t *testing.T) {
	ts, logPath := newFakebackWithRequestLog(t, reqLogDB)
	resp, err := http.Get(ts.URL + "/boards/b1")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	resp2, _ := http.Post(ts.URL+"/boards", "application/json", strings.NewReader(`{"title":"New"}`))
	resp2.Body.Close()

	lines := readLogLines(t, logPath)
	if len(lines) != 2 {
		t.Fatalf("got %d log lines, want 2", len(lines))
	}
	if lines[0]["request"].(map[string]any)["method"] != "GET" {
		t.Errorf("line 1 method = %v, want GET", lines[0]["request"])
	}
	if lines[1]["request"].(map[string]any)["method"] != "POST" {
		t.Errorf("line 2 method = %v, want POST", lines[1]["request"])
	}
}

func TestRequestLog_CapturesRequestAndResponseBodies(t *testing.T) {
	ts, logPath := newFakebackWithRequestLog(t, reqLogDB)
	body := strings.NewReader(`{"title":"Walk dog"}`)
	resp, err := http.Post(ts.URL+"/boards", "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	lines := readLogLines(t, logPath)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	req := lines[0]["request"].(map[string]any)
	reqBody := req["body"].(map[string]any)
	if reqBody["title"] != "Walk dog" {
		t.Errorf("request body title = %v, want Walk dog", reqBody["title"])
	}
	resp1 := lines[0]["response"].(map[string]any)
	if int(resp1["status"].(float64)) != http.StatusCreated {
		t.Errorf("response status = %v, want 201", resp1["status"])
	}
	respBody := resp1["body"].(map[string]any)
	if respBody["title"] != "Walk dog" {
		t.Errorf("response body title = %v, want Walk dog (echoed by CRUD)", respBody["title"])
	}
}

func TestRequestLog_SkipsHealth(t *testing.T) {
	ts, logPath := newFakebackWithRequestLog(t, reqLogDB)
	resp, _ := http.Get(ts.URL + "/health")
	resp.Body.Close()
	resp2, _ := http.Get(ts.URL + "/healthz")
	resp2.Body.Close()
	// Hit a real endpoint too so the file isn't empty.
	resp3, _ := http.Get(ts.URL + "/boards/b1")
	resp3.Body.Close()

	lines := readLogLines(t, logPath)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1 (health probes should be skipped)", len(lines))
	}
	if lines[0]["request"].(map[string]any)["path"] != "/boards/b1" {
		t.Errorf("logged path = %v, want /boards/b1", lines[0]["request"])
	}
}

func TestRequestLog_CapturesDurationMS(t *testing.T) {
	ts, logPath := newFakebackWithRequestLog(t, reqLogDB)
	resp, _ := http.Get(ts.URL + "/boards/b1")
	resp.Body.Close()
	lines := readLogLines(t, logPath)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if _, ok := lines[0]["duration_ms"].(float64); !ok {
		t.Errorf("duration_ms missing or wrong type: %v", lines[0]["duration_ms"])
	}
}

func TestRequestLog_TimestampIsRFC3339Like(t *testing.T) {
	ts, logPath := newFakebackWithRequestLog(t, reqLogDB)
	resp, _ := http.Get(ts.URL + "/boards/b1")
	resp.Body.Close()
	lines := readLogLines(t, logPath)
	tsField, _ := lines[0]["ts"].(string)
	if _, err := time.Parse("2006-01-02T15:04:05.000000Z", tsField); err != nil {
		t.Errorf("ts = %q failed to parse: %v", tsField, err)
	}
}

func TestRequestLog_StampsFakeBackendHeaderInLoggedResponse(t *testing.T) {
	ts, logPath := newFakebackWithRequestLog(t, reqLogDB)
	resp, _ := http.Get(ts.URL + "/boards/b1")
	resp.Body.Close()
	lines := readLogLines(t, logPath)
	respHeaders, _ := lines[0]["response"].(map[string]any)["headers"].(map[string]any)
	if respHeaders["X-Tavora-Fake-Backend"] != "true" {
		t.Errorf("X-Tavora-Fake-Backend = %v in logged headers; should be stamped", respHeaders["X-Tavora-Fake-Backend"])
	}
}

func TestRequestLog_RedactsSensitiveHeadersByDefault(t *testing.T) {
	ts, logPath := newFakebackWithRequestLog(t, reqLogDB)
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/boards/b1", nil)
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiJ9.payload.sig")
	req.Header.Set("Cookie", "session=very-secret")
	req.Header.Set("X-Api-Key", "tvr_abcdef")
	req.Header.Set("X-Public", "public-value")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	lines := readLogLines(t, logPath)
	reqHeaders := lines[0]["request"].(map[string]any)["headers"].(map[string]any)
	if reqHeaders["Authorization"] != "<redacted>" {
		t.Errorf("Authorization not redacted: %v", reqHeaders["Authorization"])
	}
	if reqHeaders["Cookie"] != "<redacted>" {
		t.Errorf("Cookie not redacted: %v", reqHeaders["Cookie"])
	}
	if reqHeaders["X-Api-Key"] != "<redacted>" {
		t.Errorf("X-Api-Key not redacted: %v", reqHeaders["X-Api-Key"])
	}
	// Non-sensitive headers pass through.
	if reqHeaders["X-Public"] != "public-value" {
		t.Errorf("X-Public was redacted; should not be: %v", reqHeaders["X-Public"])
	}
}

func TestRequestLog_CustomRedactList(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "db.json"), []byte(reqLogDB), 0o644); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	st, _ := store.Open(store.Config{File: filepath.Join(dir, "db.json"), IDStrategy: "int"}, logger)
	defer st.Close()

	logPath := filepath.Join(dir, "requests.jsonl")
	reqLog, err := requestlog.Open(requestlog.Options{
		Path:          logPath,
		RedactHeaders: []string{"X-Custom"}, // only this; Authorization is NOT redacted now
	}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer reqLog.Close()

	h, _ := server.New(server.Options{Logger: logger, Store: st, RequestLog: reqLog})
	ts := httptest.NewServer(h)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/boards/b1", nil)
	req.Header.Set("Authorization", "Bearer eyJ.token.sig")
	req.Header.Set("X-Custom", "secret-thing")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()

	lines := readLogLines(t, logPath)
	reqHeaders := lines[0]["request"].(map[string]any)["headers"].(map[string]any)
	if reqHeaders["X-Custom"] != "<redacted>" {
		t.Errorf("custom-redacted header X-Custom not redacted: %v", reqHeaders["X-Custom"])
	}
	if reqHeaders["Authorization"] != "Bearer eyJ.token.sig" {
		t.Errorf("Authorization unexpectedly redacted (custom list overrode default): %v", reqHeaders["Authorization"])
	}
}

func TestRequestLog_RotationKicksInAtMaxSize(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "db.json"), []byte(reqLogDB), 0o644); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	st, _ := store.Open(store.Config{File: filepath.Join(dir, "db.json"), IDStrategy: "int"}, logger)
	defer st.Close()

	logPath := filepath.Join(dir, "requests.jsonl")
	// MaxSizeMB=1 → rotation after ~1MB of writes.
	reqLog, err := requestlog.Open(requestlog.Options{
		Path:       logPath,
		MaxSizeMB:  1,
		MaxBackups: 2,
	}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer reqLog.Close()

	h, _ := server.New(server.Options{Logger: logger, Store: st, RequestLog: reqLog})
	ts := httptest.NewServer(h)
	defer ts.Close()

	// Hit /boards (returns a 60-ish-byte body) ~3,000 times → ~1.5MB
	// of log lines. Lumberjack rotates partway through. Body decode
	// inflates each line beyond the raw response, so this comfortably
	// crosses the 1MB threshold.
	for i := 0; i < 3000; i++ {
		resp, _ := http.Get(ts.URL + "/boards")
		_, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
	}

	// Inspect the directory for backup files. Lumberjack names them
	// requests-<timestamp>.jsonl (or .jsonl.gz if Compress is on).
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var backups int
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "requests-") && strings.HasSuffix(name, ".jsonl") {
			backups++
		}
	}
	if backups < 1 {
		t.Errorf("expected at least 1 rotated backup, got %d in %v",
			backups, dirNames(entries))
	}
}

func dirNames(es []os.DirEntry) []string {
	out := make([]string, 0, len(es))
	for _, e := range es {
		out = append(out, e.Name())
	}
	return out
}

func TestRequestLog_ConcurrentRequestsDoNotInterleaveLines(t *testing.T) {
	ts, logPath := newFakebackWithRequestLog(t, reqLogDB)
	var wg sync.WaitGroup
	const N = 25
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, _ := http.Get(ts.URL + "/boards/b1")
			resp.Body.Close()
		}()
	}
	wg.Wait()
	lines := readLogLines(t, logPath)
	if len(lines) != N {
		t.Errorf("got %d lines, want %d (concurrent writes must each land on their own line)", len(lines), N)
	}
}
