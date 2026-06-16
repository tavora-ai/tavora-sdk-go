// Package server wires the chi router with all routes the mock
// exposes. Routes are hand-written (one handler per endpoint) because
// the Tavora surface is small and bounded — generic CRUD on arbitrary
// keys (the half of json-server-go we forked from) doesn't apply.
//
// v0.1 mounts stub routes that return 200 + an empty shape from
// `internal/mock/canned`. v0.2 adds the SSE `/api/sdk/agents/{id}/run`
// endpoint and the scenarios machinery.
package server

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/mock/admin"
	mockauth "github.com/tavora-ai/tavora-sdk-go/cli/internal/mock/auth"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/mock/ids"
	mw "github.com/tavora-ai/tavora-sdk-go/cli/internal/mock/middleware"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/mock/scenarios"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/mock/store"
)

// Options configures the mock server.
type Options struct {
	// Logger is the slog logger handlers and middleware use. Required.
	Logger *slog.Logger
	// CORS, if true, allows browser-origin requests for any origin.
	// Useful when the mock is paired with a local web UI for demos.
	CORS bool
	// DefaultScenario, if set, overrides the per-process default
	// scenario name (otherwise scenarios.DefaultName). Per-request
	// `?scenario=<name>` query parameters still take precedence.
	DefaultScenario string
	// Seed makes generated UUIDs deterministic — two processes started
	// with the same Seed (and the same scenario) produce the same
	// trace. Defaults to 0, which still seeds the ID source but uses a
	// fixed start counter — fine for tests, less fine when two
	// processes need to differ. Pass distinct seeds for distinct
	// per-process IDs.
	Seed int64
}

// New builds the http.Handler that the mock binary serves. The store
// is created internally and exposed only via /_admin/* — for v0.1
// nothing else needs to read or write it.
func New(opts Options) (http.Handler, error) {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	registry, err := scenarios.LoadEmbedded()
	if err != nil {
		return nil, err
	}
	st := store.New()
	mgr := mockauth.NewManager("", "tavora-mock", 0)

	r := chi.NewRouter()
	r.Use(mw.RequestID)
	r.Use(mw.Recoverer(opts.Logger))
	r.Use(mw.Logger(opts.Logger))
	r.Use(mw.MockHeader)

	if opts.CORS {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   []string{"*"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Request-Id", "X-API-Key", "X-Tavora-Deployment"},
			ExposedHeaders:   []string{"X-Request-Id", mw.MockHeaderName, "Location"},
			AllowCredentials: false,
			MaxAge:           300,
		}))
	}

	idSource := ids.New(opts.Seed)
	h := newHandlers(st, mgr, registry, idSource, opts.DefaultScenario)
	a := admin.New(st)

	// Liveness / version
	r.Get("/health", h.Health)
	r.Get("/healthz", h.Health)
	r.Get("/api/version", h.Version)

	// Auth — issues a JWT for any (username, password) pair.
	r.Post("/auth/login", h.Login)

	// Admin
	r.Post("/_admin/reset", a.Reset)
	r.Get("/_admin/snapshot", a.Snapshot)

	// SDK surface. v0.1 routes return canned shapes from the
	// internal/mock/canned package; v0.2 layers SSE on top.
	r.Route("/api/sdk", func(r chi.Router) {
		r.Get("/project", h.GetProject)
		r.Post("/project/seed", h.SeedProject)
		r.Get("/capabilities", h.ListCapabilities)

		// source-* — the code-first round-trip (tavora dev / deploy).
		r.Put("/source-sync", h.SourceSync)
		r.Post("/source-validate", h.SourceValidate)
		r.Post("/source-deploy", h.SourceDeploy)
		r.Get("/source-export", h.SourceExport)
		r.Post("/source-rename", h.SourceRename)
		r.Post("/source-delete", h.SourceDelete)
		r.Post("/source-diff", h.SourceDiff)

		// Deployment env (KV/secrets per deployment slug).
		r.Get("/deployments/{slug}/env", h.ListDeploymentEnv)
		r.Get("/deployments/{slug}/env/{key}", h.GetDeploymentEnv)
		r.Put("/deployments/{slug}/env/{key}", h.PutDeploymentEnv)
		r.Delete("/deployments/{slug}/env/{key}", h.DeleteDeploymentEnv)

		// Agent sessions. /run lands in v0.2 (SSE).
		r.Post("/agents", h.CreateAgentSession)
		r.Get("/agents", h.ListAgentSessions)
		r.Get("/agents/{id}", h.GetAgentSession)
		r.Delete("/agents/{id}", h.DeleteAgentSession)
		r.Post("/agents/{id}/run", h.RunAgent)
		r.Post("/agents/{id}/input", h.RespondAgentInput)
		r.Get("/agents/system-prompt", h.GetAgentSystemPrompt)
	})

	// Admin-style introspection for scenarios.
	r.Get("/_admin/scenarios", h.ListScenarios)

	// Default for unmounted SDK paths: 501 with a structured body so a
	// client gets a clear "this endpoint isn't mocked yet" signal
	// instead of a 404 that could be confused for a routing bug.
	r.NotFound(h.NotFound)

	return r, nil
}
