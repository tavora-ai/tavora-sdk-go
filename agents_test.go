package tavora

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestCreateAgentSession(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/agents", 201, AgentSession{
		ID:     "as_1",
		Title:  "Research Agent",
		Status: "active",
	})

	session, err := ts.client().CreateAgentSession(context.Background(), CreateAgentSessionInput{
		Title: "Research Agent",
		Tools: []string{"search", "remember"},
	})
	assertNoError(t, err)
	assertEqual(t, "id", session.ID, "as_1")
	assertEqual(t, "status", session.Status, "active")

	req := ts.lastRequest(t)
	var body CreateAgentSessionInput
	json.Unmarshal([]byte(req.Body), &body)
	assertEqual(t, "title", body.Title, "Research Agent")
	assertEqual(t, "tools count", len(body.Tools), 2)
}

func TestListAgentSessions(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/agents", 200, map[string]interface{}{
		"sessions": []AgentSession{
			{ID: "as_1", Title: "Agent 1"},
		},
	})

	sessions, err := ts.client().ListAgentSessions(context.Background(), 10, 0)
	assertNoError(t, err)
	assertEqual(t, "count", len(sessions), 1)

	req := ts.lastRequest(t)
	assertEqual(t, "path", req.Path, "/api/sdk/agents?limit=10&offset=0")
}

func TestListAgentSessions_DefaultLimit(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/agents", 200, map[string]interface{}{
		"sessions": []AgentSession{},
	})

	_, err := ts.client().ListAgentSessions(context.Background(), 0, 0)
	assertNoError(t, err)

	req := ts.lastRequest(t)
	assertEqual(t, "path", req.Path, "/api/sdk/agents?limit=50&offset=0")
}

func TestGetAgentSession(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/agents/as_1", 200, AgentSessionDetail{
		Session: AgentSession{ID: "as_1"},
		Steps: []AgentStep{
			{ID: "step_1", StepType: "thinking", Content: "Let me search..."},
			{ID: "step_2", StepType: "tool_call", ToolName: "search"},
		},
	})

	detail, err := ts.client().GetAgentSession(context.Background(), "as_1")
	assertNoError(t, err)
	assertEqual(t, "session id", detail.Session.ID, "as_1")
	assertEqual(t, "steps", len(detail.Steps), 2)
	assertEqual(t, "step type", detail.Steps[1].StepType, "tool_call")
}

func TestDeleteAgentSession(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodDelete, "/api/sdk/agents/as_1", 204, nil)

	err := ts.client().DeleteAgentSession(context.Background(), "as_1")
	assertNoError(t, err)

	req := ts.lastRequest(t)
	assertEqual(t, "path", req.Path, "/api/sdk/agents/as_1")
}

func TestRunAgent_SSE(t *testing.T) {
	ts := newTestServer(t)

	sseData := fmt.Sprintf(
		"event: step\ndata: %s\n\nevent: done\ndata: %s\n\n",
		`{"type":"thinking","content":"Searching..."}`,
		`{"type":"done","summary":{"session_id":"as_1","steps":2,"tokens":{"prompt":100,"completion":50}}}`,
	)
	ts.onRaw(http.MethodPost, "/api/sdk/agents/as_1/run", 200, sseData)

	var events []AgentEvent
	err := ts.client().RunAgent(context.Background(), "as_1", "Find docs", func(e AgentEvent) {
		events = append(events, e)
	})
	assertNoError(t, err)
	assertEqual(t, "event count", len(events), 2)
	assertEqual(t, "first type", events[0].Type, "thinking")
	assertEqual(t, "first content", events[0].Content, "Searching...")
	assertEqual(t, "last type", events[1].Type, "done")
	if events[1].Summary == nil {
		t.Fatal("expected summary in done event")
	}
	assertEqual(t, "summary steps", events[1].Summary.Steps, 2)
}

func TestRunAgent_ServerError(t *testing.T) {
	ts := newTestServer(t)
	ts.onRaw(http.MethodPost, "/api/sdk/agents/as_1/run", 500,
		`{"message":"internal error"}`)

	err := ts.client().RunAgent(context.Background(), "as_1", "test", func(e AgentEvent) {
		t.Fatal("should not receive events on error")
	})
	assertError(t, err)
}

// A3: errors on the SSE start route now flow through parseAPIError so
// structured fields (Code, Details) survive instead of being dropped.
func TestRunAgent_StructuredError(t *testing.T) {
	ts := newTestServer(t)
	ts.onRaw(http.MethodPost, "/api/sdk/agents/as_1/run", 429,
		`{"code":"rate_limited","message":"too many runs","retry_after_seconds":30}`)

	err := ts.client().RunAgent(context.Background(), "as_1", "test", func(AgentEvent) {})
	assertError(t, err)

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	assertEqual(t, "code", apiErr.Code, "rate_limited")
	if v, ok := apiErr.Details["retry_after_seconds"]; !ok || v != float64(30) {
		t.Errorf("expected Details.retry_after_seconds=30, got %v", apiErr.Details)
	}
}

// A1: per-step Tokens populated.
func TestRunAgent_PerStepTokens(t *testing.T) {
	ts := newTestServer(t)
	sseData := "event: step\ndata: " +
		`{"type":"think","content":"...","tokens":{"prompt":42,"completion":13}}` +
		"\n\nevent: done\ndata: " +
		`{"type":"done","summary":{"session_id":"as_1","steps":1,"tokens":{"prompt":42,"completion":13}}}` +
		"\n\n"
	ts.onRaw(http.MethodPost, "/api/sdk/agents/as_1/run", 200, sseData)

	var got []AgentEvent
	err := ts.client().RunAgent(context.Background(), "as_1", "go", func(e AgentEvent) {
		got = append(got, e)
	})
	assertNoError(t, err)
	if got[0].Tokens == nil {
		t.Fatal("expected Tokens on think event; got nil — server emits this and SDK must round-trip")
	}
	assertEqual(t, "prompt tokens", got[0].Tokens.Prompt, int32(42))
	assertEqual(t, "completion tokens", got[0].Tokens.Completion, int32(13))
}

// A2: input_request extraction + RespondToAgentInput.
func TestAgentInput_RequestExtractionAndResponse(t *testing.T) {
	evt := AgentEvent{
		Type:    EventTypeInputRequest,
		Content: "Pick a flavor",
		Args: map[string]any{
			"request_id":  "req_abc",
			"input_type":  "choice",
			"message":     "Pick a flavor",
			"options":     []any{"vanilla", "chocolate"},
			"placeholder": "your pick",
		},
	}
	req := evt.AsInputRequest()
	if req == nil {
		t.Fatal("expected typed InputRequest, got nil")
	}
	assertEqual(t, "request id", req.RequestID, "req_abc")
	assertEqual(t, "input type", req.InputType, "choice")
	assertEqual(t, "options count", len(req.Options), 2)
	assertEqual(t, "first option", req.Options[0], "vanilla")
	assertEqual(t, "placeholder", req.Placeholder, "your pick")

	// Non-input events return nil so consumer code can branch cleanly.
	if (AgentEvent{Type: EventTypeResponse}).AsInputRequest() != nil {
		t.Error("AsInputRequest should return nil for non-input events")
	}

	// Round-trip the response endpoint.
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/agents/as_1/input", 200, map[string]any{"ok": true})
	err := ts.client().RespondToAgentInput(context.Background(), "as_1", "req_abc", "vanilla")
	assertNoError(t, err)

	var body map[string]any
	json.Unmarshal([]byte(ts.lastRequest(t).Body), &body)
	assertEqual(t, "request_id sent", body["request_id"], "req_abc")
	assertEqual(t, "value sent", body["value"], "vanilla")
}

// IsTerminal helper short-circuits the consumer's loop without
// stringly-typed comparisons.
func TestAgentEvent_IsTerminal(t *testing.T) {
	cases := map[string]bool{
		EventTypeDone:            true,
		EventTypeError:           true,
		EventTypeSandboxEvent:    false,
		EventTypeExecuteJS:       false,
		EventTypeExecuteJSResult: false,
		EventTypeDataUpdate:      false,
		EventTypeResponse:        false,
		EventTypeInputRequest:    false,
		"future_unknown":         false, // unknown is non-terminal so future server types don't break loops
	}
	for typ, want := range cases {
		got := AgentEvent{Type: typ}.IsTerminal()
		if got != want {
			t.Errorf("IsTerminal(%q): got %v, want %v", typ, got, want)
		}
	}
}
