// Package scenarios loads canned SSE conversations that the mock
// streams from `POST /api/sdk/agents/{id}/run`. Each scenario is a
// YAML file with a `sse:` list — one entry per event the mock emits,
// with an optional delay before each.
//
// v0.2 ships one scenario (happy-path). v0.4 adds the canonical five
// plus the `?scenario=<name>` selector and the `--seed` flag.
package scenarios

import (
	"embed"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed *.yaml
var embedded embed.FS

// DefaultName is the scenario selected when no `?scenario=...` query
// parameter or `--scenario` flag overrides it.
const DefaultName = "happy-path"

// Scenario is one canned SSE conversation.
type Scenario struct {
	Name        string  `yaml:"name"`
	Description string  `yaml:"description"`
	// Status is the HTTP status the run endpoint sets before streaming.
	// Defaults to 200; scenarios that pin a non-stream error response
	// (service-not-configured, unresolved-template, legacy-key) set
	// this to 4xx/5xx and use BodyJSON instead of SSE.
	Status int `yaml:"status,omitempty"`
	// BodyJSON, when set, is rendered as a single JSON body and the
	// SSE list is ignored — used for error-shape scenarios.
	BodyJSON map[string]any `yaml:"body,omitempty"`
	// Events is the canned SSE timeline. Each event waits Delay before
	// being written; Type appears as the SSE event name; Data is the
	// JSON payload of the `data:` line.
	Events []Event `yaml:"sse,omitempty"`
}

// Event is one canned SSE message.
type Event struct {
	// Delay before this event is emitted. Cumulative within a scenario
	// — e.g. two 200ms events run at 200ms and 400ms from stream start.
	Delay time.Duration `yaml:"delay"`
	// Type is the SSE `event:` name. Matches one of tavora-sdk-go's
	// EventType* constants (sandbox_event, execute_js, response, done,
	// etc.).
	Type string `yaml:"type"`
	// Data is the raw JSON payload written on the `data:` line. The
	// agent event wire shape (tavora-sdk-go.AgentEvent) is what
	// callers parse, so include `type` here too if downstream code
	// inspects it. The scenarios machinery does NOT automatically
	// inject Type into Data.
	Data map[string]any `yaml:"data,omitempty"`
}

// Registry is a name→Scenario map loaded once at startup.
type Registry struct {
	byName map[string]*Scenario
}

// LoadEmbedded reads every *.yaml under the package's embedded FS and
// returns a Registry. Returns an error if any file fails to parse,
// since a broken scenario should fail the build/boot rather than
// surface as "the mock returned a 404" in a downstream test.
func LoadEmbedded() (*Registry, error) {
	r := &Registry{byName: make(map[string]*Scenario)}
	entries, err := embedded.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("read embedded scenarios: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		f, err := embedded.Open(e.Name())
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", e.Name(), err)
		}
		raw, err := io.ReadAll(f)
		_ = f.Close()
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		var s Scenario
		if err := yaml.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf("parse %s: %w", e.Name(), err)
		}
		if s.Name == "" {
			// Fall back to filename without ".yaml" so dropping a file
			// without a `name:` field still gets registered.
			s.Name = strings.TrimSuffix(path.Base(e.Name()), ".yaml")
		}
		if s.Status == 0 {
			s.Status = 200
		}
		r.byName[s.Name] = &s
	}
	if len(r.byName) == 0 {
		return nil, fmt.Errorf("no scenarios embedded")
	}
	return r, nil
}

// Get returns the named scenario or nil.
func (r *Registry) Get(name string) *Scenario {
	if r == nil {
		return nil
	}
	if name == "" {
		name = DefaultName
	}
	return r.byName[name]
}

// Names returns the registered scenario names in sorted order. Useful
// for `--help` and for the eventual `GET /_admin/scenarios` endpoint.
func (r *Registry) Names() []string {
	if r == nil {
		return nil
	}
	out := make([]string, 0, len(r.byName))
	for k := range r.byName {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
