// Package scenarios injects latency, blanket errors, and per-route
// overrides into responses so skill authors can test how their
// agents react to non-happy backend behaviour. Scenarios are loaded
// from `<mock-root>/scenarios/*.yaml` on disk (not embedded —
// they're user-owned).
//
// Schema:
//
//	name: card-validation-fail
//	description: POST /cards always 422.
//	latency: 200ms                    # optional, applies to every response
//	error:                            # optional blanket — every response returns this
//	  status: 401
//	  body: { code: auth_expired }
//	overrides:                        # per-route overrides win over blanket
//	  - method: POST
//	    path: /cards
//	    status: 422
//	    body:
//	      code: validation_failed
//	      message: title required
//
// Selection precedence (per request): `?scenario=<name>` query >
// `POST /_admin/scenarios/activate` process-wide selection >
// `--scenario` startup flag > nothing (pass through).
//
// Unknown scenario names return 400 `unknown_scenario` so a typo
// doesn't silently land in pass-through.
package scenarios

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v3"
)

// Scenario is one canned response-shape variation.
type Scenario struct {
	Name        string        `yaml:"name"`
	Description string        `yaml:"description"`
	Latency     time.Duration `yaml:"latency"`
	Error       *Response     `yaml:"error,omitempty"`
	Overrides   []Override    `yaml:"overrides,omitempty"`
}

// Response is a canned HTTP response body + status + headers.
type Response struct {
	Status  int               `yaml:"status"`
	Body    any               `yaml:"body,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`
}

// Override is a per-(method, path) canned response. Path supports
// chi-style `{param}` segments.
type Override struct {
	Method   string            `yaml:"method"`
	Path     string            `yaml:"path"`
	Status   int               `yaml:"status"`
	Body     any               `yaml:"body,omitempty"`
	Headers  map[string]string `yaml:"headers,omitempty"`
	compiled *regexp.Regexp    `yaml:"-"`
}

// Registry holds parsed scenarios + the process-wide active selection.
type Registry struct {
	byName map[string]*Scenario
	active atomic.Pointer[Scenario] // nil = no scenario active
}

// LoadDir walks dir for *.yaml files and parses each into the
// registry. Missing dir → empty registry (a fakeback root without
// scenarios/ is a valid configuration).
func LoadDir(dir string) (*Registry, error) {
	r := &Registry{byName: make(map[string]*Scenario)}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return r, nil
		}
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		s, err := loadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		if s.Name == "" {
			s.Name = strings.TrimSuffix(e.Name(), ".yaml")
		}
		r.byName[s.Name] = s
	}
	return r, nil
}

func loadFile(path string) (*Scenario, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var s Scenario
	if err := yaml.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for i := range s.Overrides {
		re, err := compilePath(s.Overrides[i].Path)
		if err != nil {
			return nil, fmt.Errorf("%s: override %d path %q: %w", path, i, s.Overrides[i].Path, err)
		}
		s.Overrides[i].compiled = re
	}
	return &s, nil
}

// compilePath turns a chi-style pattern (/lists/{listId}/cards) into
// an anchored regexp. {param} matches one path segment ([^/]+).
var paramRE = regexp.MustCompile(`\{[^}]+\}`)

func compilePath(p string) (*regexp.Regexp, error) {
	parts := paramRE.Split(p, -1)
	var sb strings.Builder
	sb.WriteString("^")
	for i, lit := range parts {
		sb.WriteString(regexp.QuoteMeta(lit))
		if i < len(parts)-1 {
			sb.WriteString(`[^/]+`)
		}
	}
	sb.WriteString("$")
	return regexp.Compile(sb.String())
}

// Activate sets the process-wide active scenario. Pass "" to clear.
func (r *Registry) Activate(name string) error {
	if name == "" {
		r.active.Store(nil)
		return nil
	}
	s := r.byName[name]
	if s == nil {
		return fmt.Errorf("unknown scenario %q", name)
	}
	r.active.Store(s)
	return nil
}

// Names returns the registered scenario names, sorted.
func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.byName))
	for k := range r.byName {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ActiveName returns the process-wide active scenario name, or "".
func (r *Registry) ActiveName() string {
	s := r.active.Load()
	if s == nil {
		return ""
	}
	return s.Name
}

// resolve picks the scenario to apply for a given request: query
// parameter first, then the process-wide active.
func (r *Registry) resolve(query string) (*Scenario, error) {
	if query != "" {
		s := r.byName[query]
		if s == nil {
			return nil, fmt.Errorf("unknown scenario %q", query)
		}
		return s, nil
	}
	return r.active.Load(), nil
}

// HasAny reports whether the registry has any scenarios loaded.
// Callers can skip mounting the middleware when this is false.
func (r *Registry) HasAny() bool {
	return r != nil && len(r.byName) > 0
}

// Middleware returns a chi-compatible middleware that applies the
// active scenario's latency / overrides / blanket error.
func (r *Registry) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			s, err := r.resolve(req.URL.Query().Get("scenario"))
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{
					"code":    "unknown_scenario",
					"status":  400,
					"message": err.Error(),
					"hint":    fmt.Sprintf("known scenarios: %v", r.Names()),
				})
				return
			}
			if s == nil {
				next.ServeHTTP(w, req)
				return
			}
			if s.Latency > 0 {
				select {
				case <-time.After(s.Latency):
				case <-req.Context().Done():
					return
				}
			}
			for _, o := range s.Overrides {
				if !strings.EqualFold(o.Method, req.Method) {
					continue
				}
				if o.compiled != nil && !o.compiled.MatchString(req.URL.Path) {
					continue
				}
				if o.compiled == nil && o.Path != req.URL.Path {
					continue
				}
				writeResponse(w, o.Status, o.Body, o.Headers)
				return
			}
			if s.Error != nil {
				writeResponse(w, s.Error.Status, s.Error.Body, s.Error.Headers)
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}

func writeResponse(w http.ResponseWriter, status int, body any, headers map[string]string) {
	for k, v := range headers {
		w.Header().Set(k, v)
	}
	if status == 0 {
		status = http.StatusOK
	}
	if isEmpty(body) {
		w.WriteHeader(status)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func isEmpty(b any) bool {
	if b == nil {
		return true
	}
	if s, ok := b.(string); ok && s == "" {
		return true
	}
	return false
}
