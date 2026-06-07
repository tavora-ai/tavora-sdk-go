package server_test

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"

	"github.com/tavora-ai/tavora-sdk-go/cli/internal/mock/server"
)

// TestMockRoutesExistInSwagger walks the chi router for every route
// the mock mounts under /api/sdk/* and asserts each one has a matching
// entry in tavora-go/api/swagger.yaml.
//
// This is the v0.3 drift-detection check — it catches "mock implements
// a route the server no longer has" (the high-impact direction; the
// reverse "swagger has a route the mock doesn't implement" is by
// design and intentionally not asserted).
//
// Skipped when swagger.yaml isn't reachable (e.g. tavora-cli is being
// tested in isolation without the rest of the workspace). The
// workspace's CI runs this from the root, where the file is present.
func TestMockRoutesExistInSwagger(t *testing.T) {
	swaggerPath := findSwagger(t)
	if swaggerPath == "" {
		t.Skip("tavora-go/api/swagger.yaml not reachable; skipping drift check")
	}

	swaggerRoutes, err := parseSwaggerPaths(swaggerPath)
	if err != nil {
		t.Fatalf("parse swagger: %v", err)
	}
	if len(swaggerRoutes) == 0 {
		t.Fatalf("swagger.yaml at %s yielded zero paths", swaggerPath)
	}

	h, err := server.New(server.Options{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	router, ok := h.(chi.Router)
	if !ok {
		t.Fatalf("server.New returned %T, want chi.Router", h)
	}

	var orphans []string
	err = chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if !strings.HasPrefix(route, "/api/sdk/") {
			return nil
		}
		canonical := canonicalize(route)
		key := method + " " + canonical
		if _, ok := allowedSwaggerOrphans[key]; ok {
			return nil
		}
		if _, ok := swaggerRoutes[canonical]; ok {
			return nil
		}
		orphans = append(orphans, key)
		return nil
	})
	if err != nil {
		t.Fatalf("chi.Walk: %v", err)
	}
	if len(orphans) > 0 {
		t.Errorf("mock implements %d route(s) that swagger.yaml doesn't list "+
			"and aren't in the allowed-orphan list:\n  %s\n\n"+
			"Either add them to swagger.yaml (in tavora-go via swag annotations) or, "+
			"if intentionally missing, register them in allowedSwaggerOrphans.",
			len(orphans), strings.Join(orphans, "\n  "))
	}
}

// allowedSwaggerOrphans pins routes the mock implements that aren't
// currently in tavora-go/api/swagger.yaml. Each entry is a real gap
// in the server's swag annotation coverage, not a mock-only fiction
// — the SDK calls these routes against the live server and the
// production traffic resolves them. The entries should drain to
// empty as tavora-go closes the doc gap.
//
// To remove an entry: add the matching `// @Router /sdk/foo …`
// annotation to the server handler, regenerate swagger.yaml via
// `task be:swagger`, and delete the line here.
var allowedSwaggerOrphans = map[string]struct{}{
	// Agent run loop — SSE endpoint + paired input-response endpoint.
	"POST /api/sdk/agents/{}/run":   {}, // SSE stream; swag has limited SSE support
	"POST /api/sdk/agents/{}/input": {}, // resumes a paused run

	// System-prompt fetch — used by tavora-sdk-go.GetAgentSystemPrompt.
	"GET /api/sdk/agents/system-prompt": {},

	// Project lifecycle — seed mints the platform-invariant first agent.
	"POST /api/sdk/project/seed": {},

	// Code-first source-* family — used by `tavora dev` / `tavora deploy`.
	"PUT /api/sdk/source-sync":      {},
	"POST /api/sdk/source-validate": {},
	"POST /api/sdk/source-deploy":   {},
	"GET /api/sdk/source-export":    {},
	"POST /api/sdk/source-rename":   {},
	"POST /api/sdk/source-delete":   {},
	"POST /api/sdk/source-diff":     {},

	// Per-deployment env KV (with redaction). Used by `tavora env *`.
	"GET /api/sdk/deployments/{}/env":        {},
	"GET /api/sdk/deployments/{}/env/{}":     {},
	"PUT /api/sdk/deployments/{}/env/{}":     {},
	"DELETE /api/sdk/deployments/{}/env/{}":  {},
}

// findSwagger walks up from the test file to locate
// tavora-go/api/swagger.yaml. Returns "" if not found.
func findSwagger(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	// From tavora-cli/internal/mock/server, ../../../../tavora-go/api/swagger.yaml.
	for dir := cwd; dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, "..", "tavora-go", "api", "swagger.yaml")
		if _, err := os.Stat(candidate); err == nil {
			abs, _ := filepath.Abs(candidate)
			return abs
		}
	}
	return ""
}

// parseSwaggerPaths extracts the path entries from swagger.yaml and
// returns them keyed by (basePath + path) — the form the chi router
// also uses.
func parseSwaggerPaths(path string) (map[string]struct{}, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		BasePath string                 `yaml:"basePath"`
		Paths    map[string]any         `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(doc.Paths))
	for p := range doc.Paths {
		key := p
		// Swagger paths may or may not include the basePath; if they
		// don't, prepend it. The repo's swagger has basePath: /api and
		// most paths are bare like `/sdk/agents`, but one rebel uses
		// `/api/sdk/capabilities`.
		if !strings.HasPrefix(p, doc.BasePath+"/") && !strings.HasPrefix(p, "/api/") {
			key = doc.BasePath + p
		}
		out[canonicalize(key)] = struct{}{}
	}
	return out, nil
}

// canonicalize rewrites chi-style and swagger-style path params to a
// uniform form so set membership comparisons work. Both `{slug}` and
// `{slug:[a-z]+}` collapse to `{}`.
var paramRE = regexp.MustCompile(`\{[^}]+\}`)

func canonicalize(p string) string {
	return paramRE.ReplaceAllString(p, "{}")
}
