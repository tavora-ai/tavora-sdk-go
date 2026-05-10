package tavora

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// MCPServer represents an MCP server integration.
//
// LastTestedAt, ToolCount, and SkillEnabled are populated from the
// materialized skill row when the server has been Tested; they're zero
// values for servers that were registered but never Tested.
type MCPServer struct {
	ID           string          `json:"id"`
	ProductID  string          `json:"product_id"`
	Name         string          `json:"name"`
	URL          string          `json:"url"`
	Transport    string          `json:"transport"`
	AuthConfig   json.RawMessage `json:"auth_config"`
	Enabled      bool            `json:"enabled"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	LastTestedAt *time.Time      `json:"last_tested_at,omitempty"`
	ToolCount    int             `json:"tool_count,omitempty"`
	SkillEnabled *bool           `json:"skill_enabled,omitempty"`
}

// CreateMCPServerInput holds parameters for creating an MCP server.
type CreateMCPServerInput struct {
	Name       string          `json:"name"`
	URL        string          `json:"url"`
	Transport  string          `json:"transport,omitempty"`
	AuthConfig json.RawMessage `json:"auth_config,omitempty"`
}

// UpdateMCPServerInput holds parameters for updating an MCP server.
type UpdateMCPServerInput struct {
	Name       string          `json:"name,omitempty"`
	URL        string          `json:"url,omitempty"`
	Transport  string          `json:"transport,omitempty"`
	AuthConfig json.RawMessage `json:"auth_config,omitempty"`
	Enabled    *bool           `json:"enabled,omitempty"`
}

// ListMCPServers returns all MCP servers in the space.
func (c *Client) ListMCPServers(ctx context.Context) ([]MCPServer, error) {
	var resp struct {
		Servers []MCPServer `json:"servers"`
	}
	if err := c.get(ctx, "/api/sdk/mcp-servers", &resp); err != nil {
		return nil, err
	}
	return resp.Servers, nil
}

// GetMCPServer returns an MCP server by ID.
func (c *Client) GetMCPServer(ctx context.Context, id string) (*MCPServer, error) {
	var server MCPServer
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/mcp-servers/%s", id), &server); err != nil {
		return nil, err
	}
	return &server, nil
}

// CreateMCPServer creates a new MCP server.
func (c *Client) CreateMCPServer(ctx context.Context, input CreateMCPServerInput) (*MCPServer, error) {
	var server MCPServer
	if err := c.post(ctx, "/api/sdk/mcp-servers", input, &server); err != nil {
		return nil, err
	}
	return &server, nil
}

// UpdateMCPServer updates an MCP server by ID.
func (c *Client) UpdateMCPServer(ctx context.Context, id string, input UpdateMCPServerInput) (*MCPServer, error) {
	var server MCPServer
	if err := c.patch(ctx, fmt.Sprintf("/api/sdk/mcp-servers/%s", id), input, &server); err != nil {
		return nil, err
	}
	return &server, nil
}

// DeleteMCPServer deletes an MCP server by ID.
func (c *Client) DeleteMCPServer(ctx context.Context, id string) error {
	return c.delete(ctx, fmt.Sprintf("/api/sdk/mcp-servers/%s", id))
}

// MCPToolSchema is a snapshot of one tool an MCP server advertised at
// the time of the last successful Test. Matches the shape the server
// persists under the materialized skill row.
type MCPToolSchema struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// MCPToolChange names one tool whose shape changed since the last Test.
type MCPToolChange struct {
	Name string `json:"name"`
	What string `json:"what"`
}

// MCPToolDrift is the delta between the previously stored tool list
// and the freshly captured one. Empty slices mean "unchanged".
type MCPToolDrift struct {
	Added   []string        `json:"added"`
	Removed []string        `json:"removed"`
	Changed []MCPToolChange `json:"changed"`
}

// TestMCPServerResult is what POST /mcp-servers/{id}/test returns on
// success: the upserted skill row (as a generic map so callers don't
// need the full Skill type), the tool list that was captured, and the
// drift relative to the prior snapshot.
type TestMCPServerResult struct {
	Skill       map[string]any  `json:"skill"`
	Tools       []MCPToolSchema `json:"tools"`
	Drift       MCPToolDrift    `json:"drift"`
	IsFirstTest bool            `json:"is_first_test"`
}

// TestMCPServer dials the server, calls tools/list, and upserts a
// type='mcp' skill row with the captured schemas. Failures to dial or
// list surface as errors — no skill row is created in that case, so
// broken MCP servers show up here rather than mid-run.
func (c *Client) TestMCPServer(ctx context.Context, id string) (*TestMCPServerResult, error) {
	var out TestMCPServerResult
	if err := c.post(ctx, fmt.Sprintf("/api/sdk/mcp-servers/%s/test", id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
