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
