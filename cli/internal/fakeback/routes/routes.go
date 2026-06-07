// Package routes loads per-mock custom routes from routes.yaml and
// mounts them on a chi router ahead of the json-server CRUD layer.
//
// Use case: the skill author's backend has endpoints CRUD-over-keys
// can't express — e.g. a `POST /search?q=...` that returns a fixed
// list, or a `POST /lists/{listId}/reorder` that has no body.
// routes.yaml is the escape hatch for those.
//
// Schema (one entry per route):
//
//	routes:
//	  - method: POST
//	    path: /search
//	    status: 200
//	    body:
//	      results:
//	        - { id: "card_1", title: "Buy milk" }
//	  - method: POST
//	    path: /lists/{listId}/reorder
//	    status: 204
//	  - method: GET
//	    path: /me
//	    status: 200
//	    headers:
//	      X-Profile: full
//	    body:
//	      id: u1
//	      email: alice@example.com
//
// Custom routes are evaluated by chi's radix tree, which prefers the
// more-specific match. A literal route like `/search` wins over the
// CRUD `/{resource}` automatically.
package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"
)

// Route is one declared custom endpoint. Dual json:/yaml: tags so
// the same struct serves routes.yaml (hand-curated) and
// recorded.json (machine-captured) — the recorder writes JSON
// because the file's primary reader is an AI tool.
type Route struct {
	Method  string            `yaml:"method"          json:"method"`
	Path    string            `yaml:"path"            json:"path"`
	Status  int               `yaml:"status,omitempty" json:"status,omitempty"`
	Body    any               `yaml:"body,omitempty"   json:"body,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
}

// File is the top-level shape of both routes.yaml and recorded.json
// (same field name in either format).
type File struct {
	Routes []Route `yaml:"routes" json:"routes"`
}

// LoadFile reads and parses routes.yaml from disk. Returns an empty
// slice if the file doesn't exist (a fakeback root without
// routes.yaml is a valid configuration — pure CRUD).
func LoadFile(path string) ([]Route, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var f File
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for i, r := range f.Routes {
		if err := validate(r, i); err != nil {
			return nil, err
		}
	}
	return f.Routes, nil
}

// LoadJSONFile reads recorded.json (or any JSON-shaped routes file)
// from disk. Missing file → (nil, nil). Same File shape as
// LoadFile, so the rest of the pipeline doesn't care which format
// the routes came from.
func LoadJSONFile(path string) ([]Route, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var f File
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for i, r := range f.Routes {
		if err := validate(r, i); err != nil {
			return nil, err
		}
	}
	return f.Routes, nil
}

// LoadFolderRoutes loads routes.yaml and recorded.json from a mock
// folder and returns the merged slice. routes.yaml entries win on
// (method, path) collision — hand-curated routes always take
// precedence over captures. Either file may be missing; both
// missing → (nil, nil). chi's radix tree errors on duplicate route
// registration, so dedup happens here, not at mount time.
//
// recorded.json is JSON (not YAML) so the file is reliably parseable
// by the AI coding tools that read it most often. The unknown
// `body_raw` field on a captured non-JSON response passes through
// the JSON decoder into the Route's "ignored fields" bucket — the
// mounted handler serves only the typed Body, which is the right
// behavior (recorded.json is a starting point, not a finished mock).
func LoadFolderRoutes(folder string) ([]Route, error) {
	curated, err := LoadFile(filepath.Join(folder, "routes.yaml"))
	if err != nil {
		return nil, err
	}
	recorded, err := LoadJSONFile(filepath.Join(folder, "recorded.json"))
	if err != nil {
		return nil, err
	}
	if len(recorded) == 0 {
		return curated, nil
	}
	seen := make(map[string]struct{}, len(curated))
	for _, r := range curated {
		seen[routeKey(r)] = struct{}{}
	}
	out := append([]Route(nil), curated...)
	for _, r := range recorded {
		if _, dup := seen[routeKey(r)]; dup {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func routeKey(r Route) string {
	return strings.ToUpper(r.Method) + " " + r.Path
}

// Mount installs each Route on r. Status defaults to 200; body may be
// any JSON-serialisable value (object, array, scalar, or nil for
// no-body endpoints like 204).
func Mount(r chi.Router, list []Route) {
	for _, route := range list {
		method := strings.ToUpper(route.Method)
		status := route.Status
		if status == 0 {
			if method == http.MethodPost {
				status = http.StatusCreated
			} else if isEmptyBody(route.Body) && method != http.MethodGet {
				status = http.StatusNoContent
			} else {
				status = http.StatusOK
			}
		}
		h := handlerFor(status, route.Body, route.Headers)
		r.Method(method, route.Path, h)
	}
}

func validate(r Route, i int) error {
	if r.Method == "" {
		return fmt.Errorf("routes[%d]: method is required", i)
	}
	if r.Path == "" {
		return fmt.Errorf("routes[%d]: path is required", i)
	}
	if !strings.HasPrefix(r.Path, "/") {
		return fmt.Errorf("routes[%d]: path %q must start with /", i, r.Path)
	}
	switch strings.ToUpper(r.Method) {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
	default:
		return fmt.Errorf("routes[%d]: unknown method %q", i, r.Method)
	}
	if r.Status != 0 && (r.Status < 100 || r.Status > 599) {
		return fmt.Errorf("routes[%d]: status %d out of range", i, r.Status)
	}
	return nil
}

func handlerFor(status int, body any, headers map[string]string) http.HandlerFunc {
	// Pre-marshal the body once; routes are static, so there's no
	// reason to encode per request. Empty body (204-ish) → don't write
	// anything.
	var raw []byte
	if !isEmptyBody(body) {
		b, err := json.Marshal(body)
		if err == nil {
			raw = b
		}
	}
	return func(w http.ResponseWriter, _ *http.Request) {
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		if raw != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
		}
		w.WriteHeader(status)
		if raw != nil {
			_, _ = w.Write(raw)
		}
	}
}

func isEmptyBody(body any) bool {
	if body == nil {
		return true
	}
	if s, ok := body.(string); ok && s == "" {
		return true
	}
	return false
}
