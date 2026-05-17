package tavora

import (
	"context"
	"net/http"
	"testing"
)

// TestCreateAgentConfig / TestCreateAgentVersion_CopyOnWrite /
// TestSetActiveAgentVersion were removed on 2026-05-16 with the
// code-first pivot. TestUpdateAgentDraft / TestDiscardAgentDraft /
// TestPublishAgent / TestRevertAgent / TestRunAgentEval_Draft were
// removed 2026-05-17 when the dual-writer cleanup retired the draft
// + publish + target=draft surface from the SDK. The remaining
// read + settings + run-eval flow is covered below.

func TestListAgentConfigs(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/agent-configs", 200, []AgentConfig{
		{ID: "ag_1", Name: "One"},
		{ID: "ag_2", Name: "Two"},
	})

	list, err := ts.client().ListAgentConfigs(context.Background())
	assertNoError(t, err)
	assertEqual(t, "count", len(list), 2)
	assertEqual(t, "first id", list[0].ID, "ag_1")
}

func TestUpdateAgentSettings(t *testing.T) {
	ts := newTestServer(t)
	suite := "suite_1"
	ts.on(http.MethodPatch, "/api/sdk/agent-configs/ag_1/settings", 200, AgentConfig{
		ID:          "ag_1",
		EvalSuiteID: &suite,
	})

	cfg, err := ts.client().UpdateAgentSettings(context.Background(), "ag_1", UpdateAgentSettingsInput{
		EvalSuiteID: &suite,
	})
	assertNoError(t, err)
	if cfg.EvalSuiteID == nil || *cfg.EvalSuiteID != "suite_1" {
		t.Fatalf("expected pinned suite, got %v", cfg.EvalSuiteID)
	}
}

func TestRunAgentEval(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/agent-configs/ag_1/eval-runs", 202, EvalRunResult{
		Run: EvalRun{ID: "run_1", Status: "pending"},
	})

	res, err := ts.client().RunAgentEval(context.Background(), "ag_1")
	assertNoError(t, err)
	assertEqual(t, "run id", res.Run.ID, "run_1")
}

func TestListAgentEvalRuns(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/agent-configs/ag_1/eval-runs", 200, []EvalRun{
		{ID: "run_1", Status: "passed"},
		{ID: "run_2", Status: "failed"},
	})

	runs, err := ts.client().ListAgentEvalRuns(context.Background(), "ag_1", 10)
	assertNoError(t, err)
	assertEqual(t, "count", len(runs), 2)

	req := ts.lastRequest(t)
	if got := req.URL.Query().Get("limit"); got != "10" {
		t.Fatalf("expected limit=10 on query string, got %q", got)
	}
}
