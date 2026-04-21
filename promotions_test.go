package tavora

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestCreateSuite(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/eval-suites", 201, EvalSuite{
		ID:          "s_1",
		WorkspaceID: "ws_1",
		Name:        "Support triage",
		Threshold:   0.8,
	})

	suite, err := ts.client().CreateSuite(context.Background(), CreateSuiteInput{
		Name: "Support triage", Threshold: 0.8,
	})
	assertNoError(t, err)
	assertEqual(t, "id", suite.ID, "s_1")

	req := ts.lastRequest(t)
	var body CreateSuiteInput
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		t.Fatalf("body unmarshal: %v", err)
	}
	assertEqual(t, "name", body.Name, "Support triage")
}

func TestNewSuiteVersion_InheritsWhenCaseIDsNil(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/eval-suites/s_1/versions", 201, EvalSuiteVersion{
		ID: "sv_1", SuiteID: "s_1", Semver: "1.0.1",
	})

	v, err := ts.client().NewSuiteVersion(context.Background(), "s_1", NewSuiteVersionInput{})
	assertNoError(t, err)
	assertEqual(t, "semver", v.Semver, "1.0.1")
}

func TestProposePromotion(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/promotions", 201, AgentPromotion{
		ID: "p_1", VersionID: "v_1", Status: "pending_eval", TargetType: "api",
	})

	p, err := ts.client().ProposePromotion(context.Background(), ProposePromotionInput{
		VersionID: "v_1", TargetType: "api",
	})
	assertNoError(t, err)
	assertEqual(t, "status", p.Status, "pending_eval")
}

func TestApprovePromotion(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/promotions/p_1/approve", 200, AgentPromotion{
		ID: "p_1", Status: "deployed",
	})

	p, err := ts.client().ApprovePromotion(context.Background(), "p_1")
	assertNoError(t, err)
	assertEqual(t, "status", p.Status, "deployed")
}

func TestRejectPromotion(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/promotions/p_1/reject", 200, AgentPromotion{
		ID: "p_1", Status: "rejected", Reason: "tone is off",
	})

	p, err := ts.client().RejectPromotion(context.Background(), "p_1", "tone is off")
	assertNoError(t, err)
	assertEqual(t, "status", p.Status, "rejected")
	assertEqual(t, "reason", p.Reason, "tone is off")
}

func TestListPendingPromotions(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/promotions/pending", 200, []AgentPromotion{
		{ID: "p_1", Status: "pending_eval"},
		{ID: "p_2", Status: "pending_approval"},
	})

	list, err := ts.client().ListPendingPromotions(context.Background())
	assertNoError(t, err)
	assertEqual(t, "count", len(list), 2)
}
