package tavora

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestListMCPServers(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/mcp-servers", 200, map[string]interface{}{
		"servers": []MCPServer{
			{ID: "mcp_1", Name: "GitHub", URL: "https://github.mcp.io", Enabled: true},
		},
	})

	servers, err := ts.client().ListMCPServers(context.Background())
	assertNoError(t, err)
	assertEqual(t, "count", len(servers), 1)
	assertEqual(t, "name", servers[0].Name, "GitHub")
	assertEqual(t, "enabled", servers[0].Enabled, true)
}

func TestGetMCPServer(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/mcp-servers/mcp_1", 200, MCPServer{
		ID:   "mcp_1",
		Name: "GitHub",
		URL:  "https://github.mcp.io",
	})

	server, err := ts.client().GetMCPServer(context.Background(), "mcp_1")
	assertNoError(t, err)
	assertEqual(t, "id", server.ID, "mcp_1")
}

func TestCreateMCPServer(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/mcp-servers", 201, MCPServer{
		ID:   "mcp_new",
		Name: "Slack",
		URL:  "https://slack.mcp.io",
	})

	server, err := ts.client().CreateMCPServer(context.Background(), CreateMCPServerInput{
		Name:      "Slack",
		URL:       "https://slack.mcp.io",
		Transport: "sse",
	})
	assertNoError(t, err)
	assertEqual(t, "id", server.ID, "mcp_new")

	req := ts.lastRequest(t)
	var body CreateMCPServerInput
	json.Unmarshal([]byte(req.Body), &body)
	assertEqual(t, "transport", body.Transport, "sse")
}

func TestUpdateMCPServer(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPatch, "/api/sdk/mcp-servers/mcp_1", 200, MCPServer{
		ID:      "mcp_1",
		Enabled: false,
	})

	enabled := false
	server, err := ts.client().UpdateMCPServer(context.Background(), "mcp_1", UpdateMCPServerInput{
		Enabled: &enabled,
	})
	assertNoError(t, err)
	assertEqual(t, "enabled", server.Enabled, false)

	req := ts.lastRequest(t)
	assertEqual(t, "method", req.Method, http.MethodPatch)
}

func TestDeleteMCPServer(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodDelete, "/api/sdk/mcp-servers/mcp_1", 204, nil)

	err := ts.client().DeleteMCPServer(context.Background(), "mcp_1")
	assertNoError(t, err)
}
