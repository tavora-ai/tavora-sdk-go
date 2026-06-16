// Package server wires the chi router that tavora-fake-backend serves.
// Mounts /_admin/{reset,snapshot} and the json-server-style CRUD
// routes (GET/POST/PUT/PATCH/DELETE on /{resource}[/{id}]).
//
// v0.1 covers CRUD + admin only. v0.2 adds routes.yaml custom routes
// ahead of the CRUD layer; v0.3 adds the fetchPolicies binding
// verifier; v0.4 adds scenarios.
package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/crud"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/fetchpolicy"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/proxy"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/requestlog"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/routes"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/scenarios"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/store"
	mw "github.com/tavora-ai/tavora-sdk-go/cli/internal/mockcommon/middleware"
)

// FakeBackendHeaderName is the response header that identifies a
// tavora-fake-backend response. The sandbox's fetch shim (v0.6) will
// refuse to surface responses with this header unless
// TAVORA_ALLOW_FAKE=1 or the session is flagged dev-mode.
const FakeBackendHeaderName = "X-Tavora-Fake-Backend"

// Options configures the fakeback HTTP handler.
type Options struct {
	Logger *slog.Logger
	Store  *store.Store
	// Routes are the parsed routes.yaml entries to mount ahead of the
	// json-server CRUD layer. May be nil/empty.
	Routes []routes.Route
	// ExpectedHeaders maps header name → template (e.g.
	// "Authorization": "Bearer ${session.jwt}"). When non-empty, the
	// fetchpolicy middleware 412s every request that doesn't carry
	// the declared headers with non-empty ${...} slots. Liveness
	// and /_admin/* are exempt.
	ExpectedHeaders map[string]string
	// Scenarios holds the loaded scenario registry. Pass nil to skip
	// the scenarios middleware entirely. The middleware honors
	// `?scenario=<name>` per-request and `POST /_admin/scenarios/activate`
	// process-wide.
	Scenarios *scenarios.Registry
	// Upstream, when non-empty, is the absolute URL of an upstream
	// server to which the proxy fallback forwards any request the
	// local handlers (routes / CRUD / scenarios) return 404 for. The
	// upstream's response is mirrored back verbatim and, if Record
	// is non-nil, captured to disk. The canonical use is record-mode:
	// `tavora-fake-backend --upstream https://api.real.com --record-to recorded.json`.
	Upstream string
	// Record, when non-nil, is the proxy.RecorderFunc invoked after
	// each proxied response. Pass recorder.New(path).Func().
	Record proxy.RecorderFunc
	// RequestLog, when non-nil, appends one JSONL entry per request
	// to the file the logger was Opened on. Mounted as the outermost
	// middleware so logged entries see the final response shape
	// (including X-Tavora-Fake-Backend and fetchpolicy verdicts).
	RequestLog *requestlog.Logger
	CORS       bool
}

// New builds the http.Handler. Store must be non-nil (the CRUD layer
// has no fallback).
func New(opts Options) (http.Handler, error) {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	r := chi.NewRouter()
	r.Use(mw.RequestID)
	r.Use(mw.Recoverer(opts.Logger))
	r.Use(mw.Logger(opts.Logger))
	r.Use(mw.SelfIDHeader(FakeBackendHeaderName, "true"))
	if opts.RequestLog != nil {
		// Mounted AFTER SelfIDHeader so the captured response carries
		// the X-Tavora-Fake-Backend stamp. The buffer cost is negligible
		// for JSON traffic; SSE is unsupported in fakeback so streaming
		// is not a concern.
		r.Use(opts.RequestLog.Middleware())
	}

	if opts.CORS {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   []string{"*"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Request-Id"},
			ExposedHeaders:   []string{"X-Request-Id", FakeBackendHeaderName, "Location", "X-Total-Count", "X-Page", "X-Limit"},
			AllowCredentials: false,
			MaxAge:           300,
		}))
	}

	rest := crud.NewREST(opts.Store)

	verifier, err := fetchpolicy.New(fetchpolicy.Config{ExpectedHeaders: opts.ExpectedHeaders})
	if err != nil {
		return nil, err
	}

	var proxyHandler http.Handler
	if opts.Upstream != "" {
		p, err := proxy.New(opts.Upstream, opts.Record)
		if err != nil {
			return nil, err
		}
		proxyHandler = p
	}

	// Liveness — exempt from the fetchpolicy check so health probes
	// don't need to know what headers the agent sends.
	r.Get("/health", health)
	r.Get("/healthz", health)

	// Admin — also exempt. Operator tools (curl, the run-view's reset
	// button) won't carry the agent's session bearer.
	r.Post("/_admin/reset", adminReset(opts.Store))
	r.Get("/_admin/snapshot", adminSnapshot(opts.Store))
	r.Get("/_admin/resources", adminResources(opts.Store))
	r.Get("/_admin/scenarios", adminListScenarios(opts.Scenarios))
	r.Post("/_admin/scenarios/activate", adminActivateScenario(opts.Scenarios))
	r.Post("/_admin/scenarios/deactivate", adminDeactivateScenario(opts.Scenarios))

	// Everything skill-facing goes through the fetchpolicy verifier
	// (if any expectations were declared), then the scenarios
	// middleware (latency / overrides / blanket errors), then the
	// route layer. fetchpolicy runs first so a binding bug surfaces
	// regardless of which scenario is active.
	r.Group(func(g chi.Router) {
		if verifier.HasExpectations() {
			g.Use(verifier.Middleware())
		}
		if opts.Scenarios.HasAny() {
			g.Use(opts.Scenarios.Middleware())
		}
		// Proxy fallback: when an upstream is configured, any 404 from
		// the local handlers (unknown resource, no matching custom
		// route) replays the request against the upstream and returns
		// its response. Captured by recorder.Func() when -record-to is
		// set, so a real backend's responses land on disk for next
		// session's replay. Middleware lives inside the group so admin
		// and health stay local-only.
		if proxyHandler != nil {
			g.Use(proxyFallback(proxyHandler))
		}
		routes.Mount(g, opts.Routes)
		g.Get("/", rest.Resources)
		g.Get("/{resource}", rest.List)
		g.Get("/{resource}/{id}", rest.Get)
		g.Post("/{resource}", rest.Create)
		g.Put("/{resource}/{id}", rest.Replace)
		g.Patch("/{resource}/{id}", rest.Patch)
		g.Delete("/{resource}/{id}", rest.Delete)
	})

	// Three-segment paths (e.g. /api/users/me) don't match any chi
	// group route, so they bypass the in-group proxyFallback
	// middleware entirely and land at chi's root NotFound. Wire the
	// proxy there too so the catch-all is symmetric. Doesn't conflict
	// with /_admin or /health — chi NotFound only fires when no
	// registered route matches.
	if proxyHandler != nil {
		r.NotFound(proxyHandler.ServeHTTP)
		r.MethodNotAllowed(proxyHandler.ServeHTTP)
	}

	return r, nil
}

// proxyFallback returns a middleware that buffers the downstream
// response. If the downstream wrote a 404, the buffer is discarded
// and the request is replayed against the proxy handler. Anything
// else (200, 201, 412, 500, …) is mirrored through.
//
// The buffer is small because fakeback responses are JSON or empty;
// 4 MiB ceiling matches the proxy's body cap.
func proxyFallback(proxyHandler http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := &bufferedResponse{header: http.Header{}}
			next.ServeHTTP(rec, r)
			if rec.status == http.StatusNotFound {
				// Local couldn't handle it — forward to upstream. The
				// proxy writes its own status + body.
				proxyHandler.ServeHTTP(w, r)
				return
			}
			// Mirror the buffered response to the real writer.
			for k, vs := range rec.header {
				for _, v := range vs {
					w.Header().Add(k, v)
				}
			}
			if rec.status == 0 {
				rec.status = http.StatusOK
			}
			w.WriteHeader(rec.status)
			_, _ = w.Write(rec.body)
		})
	}
}

// bufferedResponse implements http.ResponseWriter while capturing
// the status code + body so proxyFallback can decide whether to
// replay the request. NOT used outside the proxy fallback path —
// always-on buffering would defeat streaming responses.
type bufferedResponse struct {
	header http.Header
	body   []byte
	status int
}

func (b *bufferedResponse) Header() http.Header { return b.header }

func (b *bufferedResponse) WriteHeader(code int) {
	if b.status == 0 {
		b.status = code
	}
}

func (b *bufferedResponse) Write(p []byte) (int, error) {
	if b.status == 0 {
		b.status = http.StatusOK
	}
	b.body = append(b.body, p...)
	return len(p), nil
}

func adminListScenarios(reg *scenarios.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if reg == nil {
			writeJSON(w, http.StatusOK, map[string]any{"scenarios": []string{}, "active": ""})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"scenarios": reg.Names(),
			"active":    reg.ActiveName(),
		})
	}
}

func adminActivateScenario(reg *scenarios.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name string `json:"name"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if reg == nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "no scenarios loaded"})
			return
		}
		if err := reg.Activate(body.Name); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"code":      "unknown_scenario",
				"error":     err.Error(),
				"available": reg.Names(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"active": reg.ActiveName()})
	}
}

func adminDeactivateScenario(reg *scenarios.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if reg != nil {
			_ = reg.Activate("")
		}
		writeJSON(w, http.StatusOK, map[string]any{"active": ""})
	}
}

func health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func adminReset(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if err := s.Reset(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "action": "reset"})
	}
}

func adminSnapshot(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		raw, err := s.Snapshot()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(raw)
	}
}

func adminResources(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"resources": s.Resources()})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
