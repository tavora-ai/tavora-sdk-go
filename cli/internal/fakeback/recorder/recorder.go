// Package recorder captures proxied responses to a recorded.json
// file the author (or an AI coding tool) reads back. JSON, not YAML:
// the primary reader is an LLM-driven workflow and LLMs parse JSON
// reliably. YAML's whitespace pitfalls and ambiguous booleans (the
// "Norway problem") regularly trip AI tools when the file gets
// non-trivial — recorded files routinely do.
//
// File shape mirrors routes.Route via dual json:/yaml: struct tags
// so the routes package's LoadJSONFile reads captures back as
// additional mock routes during replay. Promotion (recorded →
// hand-curated routes.yaml) is a copy-paste of one entry, not a
// `mv` — that's the cost; the read-ergonomics win pays it back
// every time an agent inspects the file.
//
// Disk writes are debounced (500ms) so a burst of recordings during
// one agent run land as a single rewrite. The whole file is
// rewritten on each flush — append-style writes would corrupt the
// JSON array, and the file is small enough that rewriting is cheap.
//
// Concurrency: Record() is safe to call from many goroutines; the
// internal map is mutex-protected.
package recorder

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/proxy"
)

// FlushDebounce is the quiet period after the last Record() call
// before the recorder rewrites the file. Matches the watch package's
// debounce so editor saves and recording flushes feel symmetrical.
const FlushDebounce = 500 * time.Millisecond

// Recorder is a sink for proxy captures. Construct with New(); call
// Func() to get a proxy.RecorderFunc to plumb into proxy.New().
type Recorder struct {
	path string
	log  *slog.Logger

	mu    sync.Mutex
	byKey map[string]entry // (method, path) → most-recent capture
	timer *time.Timer
}

// entry mirrors the routes.Route shape (re-declared locally so the
// recorder package doesn't import routes; the on-disk JSON structure
// is the thing that has to match, not the Go types).
type entry struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Status  int               `json:"status,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    any               `json:"body,omitempty"`
	// BodyRaw, when present, carries non-JSON response bytes as a
	// string. routes.yaml won't pick this up as a typed body —
	// recorded.json is a starting point, not a finished mock.
	BodyRaw string `json:"body_raw,omitempty"`
	// Query is the original request's query string (for the author's
	// reference; the replay matcher ignores it). Stripped from the
	// serialized output when empty.
	Query string `json:"query,omitempty"`
}

type fileShape struct {
	Routes []entry `json:"routes"`
}

// New returns a Recorder that flushes to path. Missing directory is
// created on first write; missing file is created. Existing file is
// not read at startup — the recorder is the writer, not a merger
// (the load step on the next process startup is what re-uses
// captures).
func New(path string, log *slog.Logger) *Recorder {
	if log == nil {
		log = slog.Default()
	}
	return &Recorder{
		path:  path,
		log:   log,
		byKey: map[string]entry{},
	}
}

// Func returns a proxy.RecorderFunc bound to this recorder. The
// proxy package calls this after every upstream response.
func (r *Recorder) Func() proxy.RecorderFunc {
	return r.Record
}

// Record captures one response. Safe for concurrent use.
func (r *Recorder) Record(c proxy.Captured) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := c.Method + " " + c.Path
	e := entry{
		Method:  strings.ToUpper(c.Method),
		Path:    c.Path,
		Status:  c.Status,
		Headers: filterReplayHeaders(c.Headers),
		Query:   c.Query,
	}
	if body := decodeBody(c.Body, c.Headers); body != nil {
		e.Body = body
	} else if len(c.Body) > 0 {
		e.BodyRaw = string(c.Body)
	}
	r.byKey[key] = e

	if r.timer == nil {
		r.timer = time.AfterFunc(FlushDebounce, func() {
			if err := r.Flush(); err != nil {
				r.log.Warn("recorder flush failed", "err", err, "path", r.path)
			}
		})
	} else {
		r.timer.Reset(FlushDebounce)
	}
}

// Flush rewrites the recorded.json file from the in-memory map. Safe
// to call manually (tests do); Record() schedules it automatically.
func (r *Recorder) Flush() error {
	r.mu.Lock()
	entries := make([]entry, 0, len(r.byKey))
	for _, e := range r.byKey {
		entries = append(entries, e)
	}
	r.mu.Unlock()

	// Stable order: method then path. The recorded file should diff
	// cleanly between sessions so the author can see what changed.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Method != entries[j].Method {
			return entries[i].Method < entries[j].Method
		}
		return entries[i].Path < entries[j].Path
	})

	raw, err := json.MarshalIndent(fileShape{Routes: entries}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal recorded.json: %w", err)
	}
	raw = append(raw, '\n') // trailing newline for diff-friendliness
	if dir := filepath.Dir(r.path); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, r.path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	r.log.Debug("recorder flushed", "path", r.path, "entries", len(entries))
	return nil
}

// Close stops the pending flush timer and writes any buffered
// captures synchronously. Safe to call multiple times.
func (r *Recorder) Close() error {
	r.mu.Lock()
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
	hasPending := len(r.byKey) > 0
	r.mu.Unlock()
	if !hasPending {
		return nil
	}
	return r.Flush()
}

// volatileHeaders are response headers that don't survive a replay
// because the replay re-frames the response from scratch. Content-
// Length is the load-bearing one — leaking a stale length crashes
// the client. Date / X-Request-Id change every request; Vary is
// only meaningful inside the upstream's response negotiation;
// X-Tavora-* headers are stamped by the local middleware. Strip
// these at record time so recorded.json carries only the durable
// header contract the agent should see.
var volatileHeaders = map[string]struct{}{
	"content-length":        {},
	"date":                  {},
	"x-request-id":          {},
	"vary":                  {},
	"x-tavora-mock":         {},
	"x-tavora-fake-backend": {},
	"connection":            {},
	"keep-alive":            {},
	"transfer-encoding":     {},
	"server":                {},
}

func filterReplayHeaders(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if _, vol := volatileHeaders[strings.ToLower(k)]; vol {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// decodeBody tries to interpret raw bytes as a JSON value the YAML
// encoder can render as a nested map/list. Returns nil if the bytes
// don't parse cleanly as JSON; caller falls back to BodyRaw.
//
// The Content-Type header is consulted as a hint, but a missing
// header doesn't disable the JSON probe — many real backends omit it.
func decodeBody(b []byte, headers map[string]string) any {
	if len(b) == 0 {
		return nil
	}
	ct := strings.ToLower(headers["Content-Type"])
	if ct != "" && !strings.Contains(ct, "json") && !strings.Contains(ct, "text") {
		return nil
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil
	}
	return v
}
