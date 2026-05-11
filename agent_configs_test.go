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

func TestUpsertAgentDeployment(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/agent-configs/ag_1/deployments", 201, AgentDeployment{
		ID:         "dep_1",
		AgentID:    "ag_1",
		VersionID:  "v_1",
		TargetType: "api",
		Status:     "healthy",
	})

	dep, err := ts.client().UpsertAgentDeployment(context.Background(), "ag_1", UpsertDeploymentInput{
		VersionID:  "v_1",
		TargetType: "api",
	})
	assertNoError(t, err)
	assertEqual(t, "status", dep.Status, "healthy")
}
