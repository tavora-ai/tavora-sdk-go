package tavora

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestListToolPolicies(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/tool-policies", 200, []ToolPolicy{
		{ID: "tp_1", ToolName: "fetch", Mode: "allow"},
		{ID: "tp_2", ToolName: "ai", Mode: "approve"},
	})

	rows, err := ts.client().ListToolPolicies(context.Background())
	assertNoError(t, err)
	assertEqual(t, "count", len(rows), 2)
	assertEqual(t, "first mode", rows[0].Mode, "allow")
}

func TestUpsertToolPolicy(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPut, "/api/sdk/tool-policies", 200, ToolPolicy{
		ID: "tp_1", ToolName: "sendEmail", Mode: "approve",
	})

	out, err := ts.client().UpsertToolPolicy(context.Background(), UpsertToolPolicyInput{
		ToolName: "sendEmail", Mode: "approve",
	})
	assertNoError(t, err)
	assertEqual(t, "mode", out.Mode, "approve")

	req := ts.lastRequest(t)
	var body UpsertToolPolicyInput
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		t.Fatalf("body unmarshal: %v", err)
	}
	assertEqual(t, "tool_name", body.ToolName, "sendEmail")
	assertEqual(t, "mode", body.Mode, "approve")
}

func TestUpsertToolPolicy_WithVersionOverride(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPut, "/api/sdk/tool-policies", 200, ToolPolicy{
		ID: "tp_1", ToolName: "fetch", Mode: "deny",
	})

	_, err := ts.client().UpsertToolPolicy(context.Background(), UpsertToolPolicyInput{
		AgentVersionID: "v_42",
		ToolName:       "fetch",
		Mode:           "deny",
		Config:         map[string]any{"allow_all": false},
	})
	assertNoError(t, err)

	req := ts.lastRequest(t)
	var body map[string]any
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		t.Fatalf("body unmarshal: %v", err)
	}
	assertEqual(t, "agent_version_id", body["agent_version_id"], "v_42")
}

func TestDeleteToolPolicy(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodDelete, "/api/sdk/tool-policies/tp_1", 200, map[string]string{"message": "policy deleted"})

	err := ts.client().DeleteToolPolicy(context.Background(), "tp_1")
	assertNoError(t, err)
	assertEqual(t, "path", ts.lastRequest(t).Path, "/api/sdk/tool-policies/tp_1")
}

func TestListPendingApprovals(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/approval-requests/pending", 200, []ApprovalRequest{
		{ID: "ar_1", ToolName: "sendEmail", Status: "pending"},
	})

	rows, err := ts.client().ListPendingApprovals(context.Background(), 0, 0)
	assertNoError(t, err)
	assertEqual(t, "count", len(rows), 1)
	assertEqual(t, "status", rows[0].Status, "pending")
}

func TestApproveApprovalRequest(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/approval-requests/ar_1/approve", 200, ApprovalRequest{
		ID: "ar_1", Status: "approved",
	})

	out, err := ts.client().ApproveApprovalRequest(context.Background(), "ar_1")
	assertNoError(t, err)
	assertEqual(t, "status", out.Status, "approved")
}

func TestRejectApprovalRequest(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/approval-requests/ar_1/reject", 200, ApprovalRequest{
		ID: "ar_1", Status: "rejected", ResolutionReason: "policy violation",
	})

	out, err := ts.client().RejectApprovalRequest(context.Background(), "ar_1", "policy violation")
	assertNoError(t, err)
	assertEqual(t, "status", out.Status, "rejected")

	req := ts.lastRequest(t)
	var body map[string]string
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		t.Fatalf("body unmarshal: %v", err)
	}
	assertEqual(t, "reason", body["reason"], "policy violation")
}
