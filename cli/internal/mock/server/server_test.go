package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tavora "github.com/tavora-ai/tavora-sdk-go"

	"github.com/tavora-ai/tavora-sdk-go/cli/internal/mock/server"
)

// newMock wraps server.New + httptest so each test gets a fresh
// instance. Returns the SDK client (already pointed at the mock) and
// the raw httptest.Server for direct HTTP calls.
//
// Sets TAVORA_ALLOW_MOCK=1 via t.Setenv so the SDK's mock-refusal
// hook lets these tests through. t.Setenv restores the prior value on
// test cleanup, so the env doesn't leak between tests.
func newMock(t *testing.T) (*tavora.Client, *httptest.Server) {
	t.Helper()
	t.Setenv("TAVORA_ALLOW_MOCK", "1")
	h, err := server.New(server.Options{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	c := tavora.NewClient(ts.URL, "tvr_mock_key_for_tests")
	return c, ts
}

func TestSDKRefusesMockWithoutOptIn(t *testing.T) {
	// Don't use newMock — we need TAVORA_ALLOW_MOCK explicitly unset
	// for this test. t.Setenv on the parent doesn't apply here.
	t.Setenv("TAVORA_ALLOW_MOCK", "")
	h, err := server.New(server.Options{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(h)
	defer ts.Close()
	c := tavora.NewClient(ts.URL, "tvr_mock_key")
	_, err = c.GetProject(context.Background())
	if err == nil {
		t.Fatal("expected SDK to refuse mock without TAVORA_ALLOW_MOCK")
	}
	if !errors.Is(err, tavora.ErrMockRefused) {
		t.Errorf("error = %v, want ErrMockRefused", err)
	}
}

func TestMockHeaderOnEveryResponse(t *testing.T) {
	_, ts := newMock(t)
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("X-Tavora-Mock"); got != "true" {
		t.Errorf("X-Tavora-Mock = %q, want %q", got, "true")
	}
}

func TestSDKGetProjectRoundTrips(t *testing.T) {
	c, _ := newMock(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	p, err := c.GetProject(ctx)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if p.Slug != "mock-project" {
		t.Errorf("Slug = %q, want %q", p.Slug, "mock-project")
	}
	if p.Name == "" {
		t.Errorf("Name is empty — canned response should populate it")
	}
}

func TestSDKListCapabilitiesRoundTrips(t *testing.T) {
	c, _ := newMock(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	caps, err := c.ListCapabilities(ctx)
	if err != nil {
		t.Fatalf("ListCapabilities: %v", err)
	}
	if len(caps) == 0 {
		t.Fatal("ListCapabilities returned no entries")
	}
	for _, cap := range caps {
		if cap.Name == "" {
			t.Errorf("capability has empty name: %+v", cap)
		}
	}
}

func TestSDKSourceSyncEchoesAgentIDs(t *testing.T) {
	c, _ := newMock(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	manifest := tavora.SourceSyncManifest{
		Project:    "mock-project",
		SourceHash: "sha256:abc",
		Agents: []tavora.SourceAgent{
			{ID: "support", SourceHash: "sha256:s1"},
			{ID: "copilot", SourceHash: "sha256:s2"},
		},
	}
	res, err := c.SourceSync(ctx, manifest)
	if err != nil {
		t.Fatalf("SourceSync: %v", err)
	}
	if res.DraftHash != "sha256:abc" {
		t.Errorf("DraftHash = %q, want %q (server should echo hash)", res.DraftHash, "sha256:abc")
	}
	if len(res.Agents) != 2 {
		t.Fatalf("Agents has %d entries, want 2", len(res.Agents))
	}
	gotLocals := map[string]bool{}
	for _, a := range res.Agents {
		gotLocals[a.LocalID] = true
		if a.AgentID == "" {
			t.Errorf("agent %q has empty AgentID", a.LocalID)
		}
	}
	for _, want := range []string{"support", "copilot"} {
		if !gotLocals[want] {
			t.Errorf("missing local id %q in response", want)
		}
	}
}

func TestSDKRunAgentStreamsHappyPath(t *testing.T) {
	c, _ := newMock(t)
	// Create a session first so the run endpoint has a target id.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	session, err := c.CreateAgentSession(ctx, tavora.CreateAgentSessionInput{Title: "test"})
	if err != nil {
		t.Fatalf("CreateAgentSession: %v", err)
	}

	var events []tavora.AgentEvent
	err = c.RunAgent(ctx, session.ID, "do the thing", func(e tavora.AgentEvent) {
		events = append(events, e)
	})
	if err != nil {
		t.Fatalf("RunAgent: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("RunAgent emitted zero events — the SSE stream did not parse")
	}
	// The happy-path scenario ends with a `done` event.
	last := events[len(events)-1]
	if last.Type != tavora.EventTypeDone {
		t.Errorf("last event type = %q, want %q (terminator)", last.Type, tavora.EventTypeDone)
	}
	// And contains a `response` event mid-stream so callers see the
	// final answer.
	var sawResponse bool
	for _, e := range events {
		if e.Type == tavora.EventTypeResponse {
			sawResponse = true
			if e.Content == "" {
				t.Errorf("response event had empty Content")
			}
		}
	}
	if !sawResponse {
		t.Errorf("happy-path did not emit a response event")
	}
}

func TestErrorShapeScenariosReturnStructuredBody(t *testing.T) {
	cases := []struct {
		scenario string
		status   int
		code     string
	}{
		{"service-not-configured", http.StatusServiceUnavailable, "service_not_configured"},
		{"unresolved-template", http.StatusUnprocessableEntity, "unresolved_template"},
		{"legacy-key", http.StatusBadRequest, "api_key_no_owner"},
		{"secret-in-context", http.StatusUnprocessableEntity, "secret_in_context"},
		{"context-invalid", http.StatusUnprocessableEntity, "context_invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.scenario, func(t *testing.T) {
			_, ts := newMock(t)
			url := ts.URL + "/api/sdk/agents/as_1/run?scenario=" + tc.scenario
			resp, err := http.Post(url, "application/json", strings.NewReader("{}"))
			if err != nil {
				t.Fatalf("POST: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.status {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.status)
			}
			var body map[string]any
			_ = json.NewDecoder(resp.Body).Decode(&body)
			if body["code"] != tc.code {
				t.Errorf("error code = %v, want %q", body["code"], tc.code)
			}
		})
	}
}

func TestLazySkillDiscoveryEmitsExpectedSequence(t *testing.T) {
	c, ts := newMock(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	session, err := c.CreateAgentSession(ctx, tavora.CreateAgentSessionInput{Title: "skill demo"})
	if err != nil {
		t.Fatalf("CreateAgentSession: %v", err)
	}
	// Bypass the SDK's RunAgent (which has no scenario knob) and hit
	// the raw endpoint so the test can pick the scenario.
	url := ts.URL + "/api/sdk/agents/" + session.ID + "/run?scenario=lazy-skill-discovery"
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(`{"message":"demo"}`))
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	got := strings.Count(string(raw), "event: execute_js\n")
	if got != 3 {
		t.Errorf("execute_js event count = %d, want 3 (skillList → skillGet → require)", got)
	}
	if !strings.Contains(string(raw), "event: done") {
		t.Errorf("scenario did not terminate with a done event")
	}
}

func TestSeededIDsAreDeterministic(t *testing.T) {
	// Two servers seeded the same way should issue the same first
	// session id. Without the seed, the IDs would still be stable
	// across servers (counter starts at 0 either way) — that's a
	// quirk of New(0); the property the seed guarantees is "two
	// processes can choose the same trace OR different traces."
	h1, err := server.New(server.Options{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Seed:   42,
	})
	if err != nil {
		t.Fatal(err)
	}
	h2, err := server.New(server.Options{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Seed:   42,
	})
	if err != nil {
		t.Fatal(err)
	}
	ts1 := httptest.NewServer(h1)
	defer ts1.Close()
	ts2 := httptest.NewServer(h2)
	defer ts2.Close()

	id1 := createSessionID(t, ts1)
	id2 := createSessionID(t, ts2)
	if id1 != id2 {
		t.Errorf("seeded session ids diverged: %q vs %q", id1, id2)
	}

	// Different seed → different id.
	h3, err := server.New(server.Options{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Seed:   7,
	})
	if err != nil {
		t.Fatal(err)
	}
	ts3 := httptest.NewServer(h3)
	defer ts3.Close()
	id3 := createSessionID(t, ts3)
	if id1 == id3 {
		t.Errorf("different seeds returned the same id (%q) — seeds are no-ops", id1)
	}
}

func createSessionID(t *testing.T, ts *httptest.Server) string {
	t.Helper()
	resp, err := http.Post(ts.URL+"/api/sdk/agents", "application/json", strings.NewReader(`{"title":"x"}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body.ID == "" {
		t.Fatalf("empty session id in response")
	}
	return body.ID
}

func TestUnknownScenarioReturns400(t *testing.T) {
	_, ts := newMock(t)
	resp, err := http.Post(ts.URL+"/api/sdk/agents/as_1/run?scenario=nope", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["code"] != "unknown_scenario" {
		t.Errorf("error code = %v, want unknown_scenario", body["code"])
	}
}

func TestAdminResetClearsState(t *testing.T) {
	_, ts := newMock(t)
	// Write an env entry.
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/sdk/deployments/prod/env/FOO",
		strings.NewReader(`{"value":"bar","is_secret":false}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	resp.Body.Close()

	// Confirm it's there.
	resp, err = http.Get(ts.URL + "/api/sdk/deployments/prod/env")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	var listed struct {
		Env []map[string]any `json:"env"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&listed)
	if len(listed.Env) != 1 {
		t.Fatalf("env count = %d before reset, want 1", len(listed.Env))
	}

	// Reset.
	resp, err = http.Post(ts.URL+"/_admin/reset", "", nil)
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	resp.Body.Close()

	// Should now be empty.
	resp, err = http.Get(ts.URL + "/api/sdk/deployments/prod/env")
	if err != nil {
		t.Fatalf("GET after reset: %v", err)
	}
	defer resp.Body.Close()
	listed.Env = nil
	_ = json.NewDecoder(resp.Body).Decode(&listed)
	if len(listed.Env) != 0 {
		t.Errorf("env count = %d after reset, want 0", len(listed.Env))
	}
}
