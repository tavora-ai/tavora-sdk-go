// Package fetchpolicy verifies that incoming requests carry the
// headers the skill's `fetchPolicies` block declared. This is the
// value-add over a generic CRUD server: the failure mode skill
// authors keep hitting in real dev — forgotten `session_vars.jwt`,
// miswritten template, fetchPolicies origin mismatch — becomes a
// loud 412 at the mock layer rather than a silent "the agent's
// fetch hung up" surprise.
//
// Templates use Tavora's existing `${...}` sigil:
//
//	"Authorization": "Bearer ${session.jwt}"
//	"X-Tenant": "${session.tenant_id}"
//	"X-Static": "fixed-value"               # no slot — exact match
//
// On an incoming request, for each declared (name, template), the
// middleware checks:
//
//  1. The named header is present and non-empty.
//  2. The literal parts of the template match the received value
//     in order (prefix + suffix).
//  3. Every `${...}` slot in the template resolves to a non-empty
//     run of characters in the received value.
//
// If any check fails, the response is 412 Precondition Failed with a
// structured body the run-view can annotate:
//
//	{
//	  "code": "fetchpolicy_unbound",
//	  "message": "expected ... but bearer slot was empty",
//	  "header": "Authorization",
//	  "expected_template": "Bearer ${session.jwt}",
//	  "received": "Bearer "
//	}
package fetchpolicy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// Config is the declared expectation table. Keys are HTTP header
// names (case-insensitive on the wire, but the response echoes them
// as declared); values are templates.
type Config struct {
	ExpectedHeaders map[string]string
}

// Verifier holds the compiled per-header matchers.
type Verifier struct {
	matchers map[string]*matcher
}

// New compiles cfg into a Verifier. An empty Config is valid — the
// middleware then passes every request through.
func New(cfg Config) (*Verifier, error) {
	v := &Verifier{matchers: make(map[string]*matcher, len(cfg.ExpectedHeaders))}
	for name, tmpl := range cfg.ExpectedHeaders {
		m, err := compile(name, tmpl)
		if err != nil {
			return nil, err
		}
		v.matchers[name] = m
	}
	return v, nil
}

// Middleware returns a chi-compatible middleware that 412s on
// missing or unbound declared headers.
func (v *Verifier) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for name, m := range v.matchers {
				got := r.Header.Get(name)
				if err := m.check(got); err != nil {
					writeUnbound(w, name, m.template, got, err.Error())
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// HasExpectations reports whether the verifier has any declared
// headers — used by callers that want to skip mounting the
// middleware when there's nothing to check.
func (v *Verifier) HasExpectations() bool {
	return v != nil && len(v.matchers) > 0
}

// ---- internal ----

type matcher struct {
	header   string
	template string
	// Compiled regexp: literal parts QuoteMeta'd, ${...} slots
	// rendered as `(.+)` (greedy match — slot can't be empty).
	re *regexp.Regexp
	// Plain — no template slots. Compare with strings.EqualFold.
	plain bool
}

var slotRE = regexp.MustCompile(`\$\{[^}]+\}`)

func compile(header, tmpl string) (*matcher, error) {
	if tmpl == "" {
		return nil, fmt.Errorf("fetchpolicy: header %q has empty template", header)
	}
	if !slotRE.MatchString(tmpl) {
		return &matcher{header: header, template: tmpl, plain: true}, nil
	}
	parts := slotRE.Split(tmpl, -1)
	var sb strings.Builder
	sb.WriteString("^")
	for i, p := range parts {
		sb.WriteString(regexp.QuoteMeta(p))
		if i < len(parts)-1 {
			// Slot — must match at least one character. Greedy is fine;
			// we only care about non-emptiness.
			sb.WriteString(`.+`)
		}
	}
	sb.WriteString("$")
	re, err := regexp.Compile(sb.String())
	if err != nil {
		return nil, fmt.Errorf("fetchpolicy: compile template for %q: %w", header, err)
	}
	return &matcher{header: header, template: tmpl, re: re}, nil
}

func (m *matcher) check(got string) error {
	if got == "" {
		return fmt.Errorf("header missing or empty")
	}
	if m.plain {
		if got != m.template {
			return fmt.Errorf("expected exact value %q, got %q", m.template, got)
		}
		return nil
	}
	if !m.re.MatchString(got) {
		return fmt.Errorf("value %q does not match template (a ${...} slot was empty or a literal mismatched)", got)
	}
	return nil
}

func writeUnbound(w http.ResponseWriter, name, tmpl, got, reason string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusPreconditionFailed)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code":              "fetchpolicy_unbound",
		"status":            http.StatusPreconditionFailed,
		"header":            name,
		"expected_template": tmpl,
		"received":          got,
		"message":           fmt.Sprintf("fetchPolicies binding for %s did not resolve: %s", name, reason),
		"hint":              "verify session_vars supply every ${session.X} referenced by the agent's fetchPolicies",
	})
}
