package tavora

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestGetStudioTrace(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/studio/as_1", 200, StudioTrace{
		Session:      AgentSession{ID: "as_1"},
		SystemPrompt: "You are a helpful agent.",
		Tools:        []string{"search", "remember"},
		Memory:       map[string]string{"key1": "value1"},
	})

	trace, err := ts.client().GetStudioTrace(context.Background(), "as_1")
	assertNoError(t, err)
	assertEqual(t, "session id", trace.Session.ID, "as_1")
	assertEqual(t, "system prompt", trace.SystemPrompt, "You are a helpful agent.")
	assertEqual(t, "tools count", len(trace.Tools), 2)
	assertEqual(t, "memory key1", trace.Memory["key1"], "value1")

	req := ts.lastRequest(t)
	assertEqual(t, "path", req.Path, "/api/sdk/studio/as_1")
}

func TestReplayFromStep_SSE(t *testing.T) {
	ts := newTestServer(t)

	sseData := fmt.Sprintf(
		"event: step\ndata: %s\n\nevent: done\ndata: %s\n\n",
		`{"type":"thinking","content":"Replaying..."}`,
		`{"type":"done","summary":{"session_id":"as_replay","steps":1,"tokens":{"prompt":50,"completion":25}}}`,
	)
	ts.onRaw(http.MethodPost, "/api/sdk/studio/as_1/replay", 200, sseData)

	var events []AgentEvent
	err := ts.client().ReplayFromStep(context.Background(), "as_1", StudioReplayConfig{
		FromStep: 2,
	}, func(e AgentEvent) {
		events = append(events, e)
	})
	assertNoError(t, err)
	assertEqual(t, "event count", len(events), 2)
	assertEqual(t, "first type", events[0].Type, "thinking")
}

func TestAnalyzeFix(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/studio/as_1/analyze", 200, StudioFixSuggestion{
		PromptChanges: "Add explicit instruction to use search tool first.",
		Reasoning:     "The agent skipped the search step.",
		EvalCase: &struct {
			Name     string `json:"name"`
			Prompt   string `json:"prompt"`
			Criteria string `json:"criteria"`
		}{
			Name:     "search-first",
			Prompt:   "Find pricing info",
			Criteria: "Must call search before responding",
		},
	})

	suggestion, err := ts.client().AnalyzeFix(context.Background(), "as_1", StudioFixRequest{
		FailedSteps:     []int{2, 3},
		ExpectedOutcome: "Should search before answering",
	})
	assertNoError(t, err)
	if suggestion.PromptChanges == "" {
		t.Error("expected prompt_changes")
	}
	if suggestion.EvalCase == nil {
		t.Fatal("expected eval_case suggestion")
	}
	assertEqual(t, "eval name", suggestion.EvalCase.Name, "search-first")

	req := ts.lastRequest(t)
	var body StudioFixRequest
	json.Unmarshal([]byte(req.Body), &body)
	assertEqual(t, "failed steps count", len(body.FailedSteps), 2)
	assertEqual(t, "expected_outcome", body.ExpectedOutcome, "Should search before answering")
}
