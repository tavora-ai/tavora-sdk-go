package tavora

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ToolPolicy is a Phase-14 primitive: a gate on a single tool for a single
// app (and optionally a single agent version). The sandbox consults
// one of these before every tool call.
//
// AgentVersionID is nil for app-default rows; set to a specific
// version's id for per-version overrides. Version overrides beat defaults.
type ToolPolicy struct {
	ID             string          `json:"id"`
	AppID    string          `json:"app_id"`
	AgentVersionID *string         `json:"agent_version_id"`
	ToolName       string          `json:"tool_name"`
	Mode           string          `json:"mode"` // allow | deny | approve
	ConfigJSON     json.RawMessage `json:"config_json"`
	CreatedBy      *string         `json:"created_by"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// ApprovalRequest is a Phase-14 primitive: a tool call parked waiting on
// a human because its resolved policy said `mode=approve`. The admin UI
// (or a webhook-driven host SaaS) flips Status from "pending" to
// "approved" / "rejected" / "expired" to unblock the agent session.
type ApprovalRequest struct {
	ID                 string          `json:"id"`
	AppID        string          `json:"app_id"`
	SessionID          *string         `json:"session_id"`
	AgentVersionID     *string         `json:"agent_version_id"`
	PolicyID           *string         `json:"policy_id"`
	ToolName           string          `json:"tool_name"`
	ArgsJSON           json.RawMessage `json:"args_json"`
	Status             string          `json:"status"` // pending | approved | rejected | expired
	ResolvedBy         *string         `json:"resolved_by"`
	ResolvedAt         *time.Time      `json:"resolved_at"`
	ResolutionReason   string          `json:"resolution_reason"`
	WebhookURL         string          `json:"webhook_url"`
	WebhookDeliveredAt *time.Time      `json:"webhook_delivered_at"`
	RequestedAt        time.Time       `json:"requested_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// --- Input types ---

// UpsertToolPolicyInput is keyed by (AppID, AgentVersionID, ToolName).
// Omit AgentVersionID to target the app-default row.
type UpsertToolPolicyInput struct {
	AgentVersionID string         `json:"agent_version_id,omitempty"`
	ToolName       string         `json:"tool_name"`
	Mode           string         `json:"mode"` // allow | deny | approve
	Config         map[string]any `json:"config,omitempty"`
}

// --- Tool policy methods ---

// ListToolPolicies returns all policies for the API-key's app
// (both app-defaults and per-version overrides).
func (c *Client) ListToolPolicies(ctx context.Context) ([]ToolPolicy, error) {
	var out []ToolPolicy
	if err := c.get(ctx, "/api/sdk/tool-policies", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpsertToolPolicy creates or updates a policy row. Concurrent upserts on
// the same (app, version, tool) are serialized by a partial unique
// index and silently overwrite — last write wins.
func (c *Client) UpsertToolPolicy(ctx context.Context, input UpsertToolPolicyInput) (*ToolPolicy, error) {
	var out ToolPolicy
	if err := c.put(ctx, "/api/sdk/tool-policies", input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteToolPolicy removes a policy row. The tool falls back to any
// app-default row (when deleting a version-override) or to the
// code default (allow for most tools, deny for fetch).
func (c *Client) DeleteToolPolicy(ctx context.Context, policyID string) error {
	return c.delete(ctx, fmt.Sprintf("/api/sdk/tool-policies/%s", policyID))
}

// --- Approval request methods ---

// ListPendingApprovals returns tool calls currently parked on an admin
// decision. Pagination via limit/offset; server caps limit at 500.
func (c *Client) ListPendingApprovals(ctx context.Context, limit, offset int) ([]ApprovalRequest, error) {
	path := "/api/sdk/approval-requests/pending"
	if limit > 0 || offset > 0 {
		path += fmt.Sprintf("?limit=%d&offset=%d", limit, offset)
	}
	var out []ApprovalRequest
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ApproveApprovalRequest unblocks the parked tool call. The agent session's
// in-flight WaitForApproval returns with Status="approved" and the tool
// proceeds. Returns BadRequest if the approval already resolved (e.g.
// another admin beat you to it or the timeout expired the request).
func (c *Client) ApproveApprovalRequest(ctx context.Context, approvalID string) (*ApprovalRequest, error) {
	var out ApprovalRequest
	if err := c.post(ctx, fmt.Sprintf("/api/sdk/approval-requests/%s/approve", approvalID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RejectApprovalRequest rejects the parked tool call with a required
// reason. The agent session's WaitForApproval returns an error; the tool
// call surfaces the reason to the reasoning trace.
func (c *Client) RejectApprovalRequest(ctx context.Context, approvalID, reason string) (*ApprovalRequest, error) {
	body := map[string]string{"reason": reason}
	var out ApprovalRequest
	if err := c.post(ctx, fmt.Sprintf("/api/sdk/approval-requests/%s/reject", approvalID), body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
