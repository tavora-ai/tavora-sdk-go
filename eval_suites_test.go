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
		ID:        "s_1",
		AppID:     "ws_1",
		Name:      "Support triage",
		Threshold: 0.8,
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
