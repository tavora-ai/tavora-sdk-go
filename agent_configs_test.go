package tavora

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestCreateAgentConfig(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/agent-configs", 201, AgentConfig{
		ID:          "ag_new",
		AppID: "ws_1",
		Name:        "Revenue Bot",
		Description: "drafts outreach",
	})

	cfg, err := ts.client().CreateAgentConfig(context.Background(), CreateAgentConfigInput{
		Name:        "Revenue Bot",
		Description: "drafts outreach",
	})
	assertNoError(t, err)
	assertEqual(t, "id", cfg.ID, "ag_new")
	assertEqual(t, "name", cfg.Name, "Revenue Bot")

	req := ts.lastRequest(t)
	assertEqual(t, "method", req.Method, http.MethodPost)
	assertEqual(t, "path", req.Path, "/api/sdk/agent-configs")
}

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

func TestCreateAgentVersion_CopyOnWrite(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/agent-configs/ag_1/versions", 201, AgentVersion{
		ID:      "v_new",
		AgentID: "ag_1",
		Semver:  "1.0.1",
	})

	v, err := ts.client().CreateAgentVersion(context.Background(), "ag_1", CreateAgentVersionInput{
		FromVersionID: "v_prev",
		PersonaMD:     "override",
	})
	assertNoError(t, err)
	assertEqual(t, "id", v.ID, "v_new")

	req := ts.lastRequest(t)
	var body CreateAgentVersionInput
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		t.Fatalf("body unmarshal: %v", err)
	}
	assertEqual(t, "from_version_id", body.FromVersionID, "v_prev")
	assertEqual(t, "persona_md", body.PersonaMD, "override")
}

func TestSetActiveAgentVersion(t *testing.T) {
	ts := newTestServer(t)
	active := "v_2"
	ts.on(http.MethodPut, "/api/sdk/agent-configs/ag_1/active-version", 200, AgentConfig{
		ID:              "ag_1",
		ActiveVersionID: &active,
	})

	cfg, err := ts.client().SetActiveAgentVersion(context.Background(), "ag_1", "v_2")
	assertNoError(t, err)
	if cfg.ActiveVersionID == nil || *cfg.ActiveVersionID != "v_2" {
		t.Fatalf("expected active_version_id=v_2, got %v", cfg.ActiveVersionID)
	}

	req := ts.lastRequest(t)
	assertEqual(t, "method", req.Method, http.MethodPut)
}

func TestUpdateAgentDraft(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPatch, "/api/sdk/agent-configs/ag_1/draft", 200, AgentConfig{
		ID:   "ag_1",
		Name: "Bot",
		DraftConfig: &DraftConfig{
			PersonaMD: "new persona",
			Model:     "gpt-4.1",
			Provider:  "openai",
		},
	})

	cfg, err := ts.client().UpdateAgentDraft(context.Background(), "ag_1", DraftConfig{
		PersonaMD: "new persona",
		Model:     "gpt-4.1",
		Provider:  "openai",
	})
	assertNoError(t, err)
	if cfg.DraftConfig == nil || cfg.DraftConfig.PersonaMD != "new persona" {
		t.Fatalf("expected staged draft, got %+v", cfg.DraftConfig)
	}

	req := ts.lastRequest(t)
	assertEqual(t, "method", req.Method, http.MethodPatch)
	var body DraftConfig
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		t.Fatalf("body unmarshal: %v", err)
	}
	assertEqual(t, "persona_md", body.PersonaMD, "new persona")
}

func TestDiscardAgentDraft(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodDelete, "/api/sdk/agent-configs/ag_1/draft", 200, AgentConfig{
		ID:          "ag_1",
		Name:        "Bot",
		DraftConfig: nil,
	})

	cfg, err := ts.client().DiscardAgentDraft(context.Background(), "ag_1")
	assertNoError(t, err)
	if cfg.DraftConfig != nil {
		t.Fatalf("expected draft cleared, got %+v", cfg.DraftConfig)
	}
}

func TestPublishAgent(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/agent-configs/ag_1/publish", 200, PublishResult{
		Agent:   AgentConfig{ID: "ag_1", PersonaMD: "shipped"},
		Version: AgentVersion{ID: "v_5", AgentID: "ag_1", Semver: "1.0.5"},
	})

	res, err := ts.client().PublishAgent(context.Background(), "ag_1")
	assertNoError(t, err)
	assertEqual(t, "semver", res.Version.Semver, "1.0.5")
	assertEqual(t, "persona", res.Agent.PersonaMD, "shipped")
}

func TestRevertAgent(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/agent-configs/ag_1/revert", 200, PublishResult{
		Agent:   AgentConfig{ID: "ag_1"},
		Version: AgentVersion{ID: "v_6", AgentID: "ag_1", Semver: "1.0.6"},
	})

	res, err := ts.client().RevertAgent(context.Background(), "ag_1", "v_3")
	assertNoError(t, err)
	assertEqual(t, "semver", res.Version.Semver, "1.0.6")

	req := ts.lastRequest(t)
	var body map[string]string
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		t.Fatalf("body unmarshal: %v", err)
	}
	assertEqual(t, "version_id", body["version_id"], "v_3")
}

func TestUpdateAgentSettings(t *testing.T) {
	ts := newTestServer(t)
	suite := "suite_1"
	yes := true
	ts.on(http.MethodPatch, "/api/sdk/agent-configs/ag_1/settings", 200, AgentConfig{
		ID:               "ag_1",
		EvalSuiteID:      &suite,
		RunEvalOnPublish: true,
	})

	cfg, err := ts.client().UpdateAgentSettings(context.Background(), "ag_1", UpdateAgentSettingsInput{
		EvalSuiteID:      &suite,
		RunEvalOnPublish: &yes,
	})
	assertNoError(t, err)
	if cfg.EvalSuiteID == nil || *cfg.EvalSuiteID != "suite_1" {
		t.Fatalf("expected pinned suite, got %v", cfg.EvalSuiteID)
	}
	if !cfg.RunEvalOnPublish {
		t.Fatalf("expected run_eval_on_publish=true")
	}
}

func TestRunAgentEval_Draft(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/agent-configs/ag_1/eval-runs", 202, EvalRunResult{
		Run: EvalRun{ID: "run_1", Status: "pending"},
	})

	res, err := ts.client().RunAgentEval(context.Background(), "ag_1", EvalTargetDraft)
	assertNoError(t, err)
	assertEqual(t, "run id", res.Run.ID, "run_1")

	req := ts.lastRequest(t)
	if got := req.URL.Query().Get("target"); got != "draft" {
		t.Fatalf("expected target=draft on query string, got %q", got)
	}
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
