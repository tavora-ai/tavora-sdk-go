// Package requestlog captures every request the fakeback handles
// into a JSONL file. Primary use case: an AI coding tool (Claude
// Code, Cursor) tails the file to see "what did the agent just do?"
// during iteration — every fetch from a skill becomes one
// inspectable, queryable line.
//
// JSONL (not JSON) because the log is append-only and tail-friendly:
// new lines appear at the bottom in real time, no whole-file
// rewrite, one bad write at most corrupts one line. The recorder
// package uses regular JSON because it's a deduplicated map; this
// package is a stream and the format reflects that.
//
// Entry shape per line:
//
//	{
//	  "ts": "2026-05-21T12:34:56.123456Z",
//	  "duration_ms": 45,
//	  "request":  {"method","path","query","headers","body"},
//	  "response": {"status","headers","body"}
//	}
//
// Bodies are decoded as nested JSON when they parse, raw string
// otherwise — matches the recorder's body handling so an AI reading
// either file gets the same shape.
//
// Concurrency: Middleware() returns a chi-compatible middleware that
// serializes one line per request through a single mutex. The
// underlying os.File is append-only.
package requestlog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// DefaultRedactHeaders is the case-insensitive set of header names
// whose values are replaced with <redacted> when written to the log.
// Customize via Options.RedactHeaders.
//
// Why redact by default: an AI coding tool tailing the log shouldn't
// have casual access to the user's bearer JWT. Operators who need
// the raw value can pass an empty Options.RedactHeaders.
var DefaultRedactHeaders = []string{
	"Authorization",
	"Cookie",
	"Set-Cookie",
	"X-Api-Key",
	"Proxy-Authorization",
}

// Options configures the request logger. Path is required; everything
// else has sane defaults for the "skill author tailing during dev"
// workflow.
type Options struct {
	// Path is the destination file. Parent directories are created.
	Path string
	// MaxSizeMB is the lumberjack rotation threshold. 0 → 50 MB.
	MaxSizeMB int
	// MaxBackups is the count of rotated files kept. 0 → 3.
	MaxBackups int
	// MaxAgeDays is the retention for rotated files. 0 → keep
	// forever; useful when a dev session drags on.
	MaxAgeDays int
	// Compress gzips rotated backups. Default off — uncompressed
	// backups stay greppable.
	Compress bool
	// RedactHeaders is the list of header names whose values are
	// replaced with <redacted> in logged entries. nil → use
	// DefaultRedactHeaders. Pass an empty slice to disable redaction
	// entirely (operator opt-in, not recommended).
	RedactHeaders []string
}

// Logger writes captured requests to a JSONL file with size-based
// rotation via lumberjack. Open with Open(); pass into
// server.Options.RequestLog.
type Logger struct {
	path    string
	log     *slog.Logger
	redact  map[string]struct{}

	mu  sync.Mutex
	wc  io.WriteCloser // lumberjack rolls the underlying file
}

// Open creates the file (and parents), wires the rotator, and
// returns a Logger ready for Middleware.
func Open(opts Options, log *slog.Logger) (*Logger, error) {
	if log == nil {
		log = slog.Default()
	}
	if opts.Path == "" {
		return nil, fmt.Errorf("requestlog: Options.Path is required")
	}
	if dir := filepath.Dir(opts.Path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("requestlog: mkdir %s: %w", dir, err)
		}
	}
	if opts.MaxSizeMB <= 0 {
		opts.MaxSizeMB = 50
	}
	if opts.MaxBackups <= 0 {
		opts.MaxBackups = 3
	}
	rotator := &lumberjack.Logger{
		Filename:   opts.Path,
		MaxSize:    opts.MaxSizeMB,
		MaxBackups: opts.MaxBackups,
		MaxAge:     opts.MaxAgeDays,
		Compress:   opts.Compress,
	}
	headers := opts.RedactHeaders
	if headers == nil {
		headers = DefaultRedactHeaders
	}
	redact := make(map[string]struct{}, len(headers))
	for _, h := range headers {
		redact[strings.ToLower(h)] = struct{}{}
	}
	return &Logger{path: opts.Path, log: log, redact: redact, wc: rotator}, nil
}

// Close flushes any in-flight writes and closes the underlying
// rotator (lumberjack closes the current file handle). Safe to call
// multiple times.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.wc == nil {
		return nil
	}
	err := l.wc.Close()
	l.wc = nil
	return err
}

// Path returns the file the logger is writing to. Useful for boot
// banners and test assertions.
func (l *Logger) Path() string { return l.path }

// Middleware returns a chi-compatible middleware that captures every
// request that flows through. Skips /health and /healthz (operator
// probes are noise). Captures /_admin/* because flipping a scenario
// is a meaningful state change the AI reader probably wants to see.
func (l *Logger) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skipPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()

			// Drain the request body once. We need to re-inject it for
			// the rest of the chain. Cap at 4 MiB to match the proxy's
			// body limit.
			var reqBody []byte
			if r.Body != nil {
				reqBody, _ = io.ReadAll(io.LimitReader(r.Body, 4<<20))
				_ = r.Body.Close()
				r.Body = io.NopCloser(bytes.NewReader(reqBody))
			}

			rec := &capturingResponse{
				ResponseWriter: w,
				body:           &bytes.Buffer{},
				status:         http.StatusOK,
			}
			next.ServeHTTP(rec, r)

			// Headers came directly from w (the underlying writer);
			// snapshot after next.ServeHTTP returns.
			respHeaders := rec.ResponseWriter.Header()
			entry := l.buildEntry(r, reqBody, rec, respHeaders, time.Since(start))
			if err := l.write(entry); err != nil {
				l.log.Warn("requestlog write failed", "err", err, "path", l.path)
			}
		})
	}
}

// skipPath suppresses logging for high-frequency, low-signal probes.
// Operator-driven /_admin/* is NOT skipped — those state changes
// are exactly what an AI reader wants context on.
func skipPath(p string) bool {
	return p == "/health" || p == "/healthz"
}

// capturingResponse wraps the downstream ResponseWriter to buffer
// the body and remember the status code, while passing writes
// through verbatim. Headers are read off the underlying writer
// after the handler returns — no custom Header() override needed.
type capturingResponse struct {
	http.ResponseWriter
	body        *bytes.Buffer
	status      int
	wroteHeader bool
}

func (c *capturingResponse) WriteHeader(code int) {
	if c.wroteHeader {
		return
	}
	c.status = code
	c.wroteHeader = true
	c.ResponseWriter.WriteHeader(code)
}

func (c *capturingResponse) Write(p []byte) (int, error) {
	if !c.wroteHeader {
		c.WriteHeader(http.StatusOK)
	}
	c.body.Write(p)
	return c.ResponseWriter.Write(p)
}

// entry is the shape of one JSONL line. Field order in struct =
// field order in serialized output, which keeps the log scannable.
type entry struct {
	TS         string          `json:"ts"`
	DurationMS int64           `json:"duration_ms"`
	Request    requestEntry    `json:"request"`
	Response   responseEntry   `json:"response"`
}

type requestEntry struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Query   string            `json:"query,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    any               `json:"body,omitempty"`
}

type responseEntry struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    any               `json:"body,omitempty"`
}

func (l *Logger) buildEntry(r *http.Request, reqBody []byte, rec *capturingResponse, respHeaders http.Header, dur time.Duration) entry {
	return entry{
		TS:         time.Now().UTC().Format("2006-01-02T15:04:05.000000Z"),
		DurationMS: dur.Milliseconds(),
		Request: requestEntry{
			Method:  r.Method,
			Path:    r.URL.Path,
			Query:   r.URL.RawQuery,
			Headers: l.flattenHeaders(r.Header),
			Body:    decodeBody(reqBody, r.Header.Get("Content-Type")),
		},
		Response: responseEntry{
			Status:  rec.status,
			Headers: l.flattenHeaders(respHeaders),
			Body:    decodeBody(rec.body.Bytes(), respHeaders.Get("Content-Type")),
		},
	}
}

func (l *Logger) write(e entry) error {
	raw, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	raw = append(raw, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.wc == nil {
		return fmt.Errorf("requestlog: file closed")
	}
	_, err = l.wc.Write(raw)
	return err
}

// flattenHeaders converts a http.Header into a flat map, applying
// redaction for any header in the configured set. Multi-value
// headers are joined with ", " before redaction so the AI reader
// still sees a single string per name.
func (l *Logger) flattenHeaders(h http.Header) map[string]string {
	if len(h) == 0 {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, vs := range h {
		if _, redacted := l.redact[strings.ToLower(k)]; redacted {
			out[k] = "<redacted>"
			continue
		}
		out[k] = strings.Join(vs, ", ")
	}
	return out
}

// decodeBody mirrors the recorder's body handling: try JSON, fall
// back to raw string. nil/empty → omitted. AI reader gets nested
// objects when the body is JSON, which is the common case.
func decodeBody(b []byte, contentType string) any {
	if len(b) == 0 {
		return nil
	}
	ct := strings.ToLower(contentType)
	if ct != "" && !strings.Contains(ct, "json") && !strings.Contains(ct, "text") {
		return string(b)
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return string(b)
	}
	return v
}
