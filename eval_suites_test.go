package tavora

import (
	"context"
	"net/http"
	"testing"
)

// CreateSuite / NewSuiteVersion tests were removed 2026-05-17 when
// the suite write surface collapsed into source-sync. Coverage for
// the upsert + version-cut path lives in tavora-go's
// TestSourceSync_UpsertsEvalCases.

func TestListSuites(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/eval-suites", 200, []EvalSuite{
		{ID: "s_1", Name: "Support triage"},
	})

	suites, err := ts.client().ListSuites(context.Background())
	assertNoError(t, err)
	assertEqual(t, "count", len(suites), 1)
	assertEqual(t, "name", suites[0].Name, "Support triage")
}

func TestListSuiteVersions(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/eval-suites/s_1/versions", 200, []EvalSuiteVersion{
		{ID: "sv_1", SuiteID: "s_1", Semver: "1.0.0"},
		{ID: "sv_2", SuiteID: "s_1", Semver: "1.0.1"},
	})

	versions, err := ts.client().ListSuiteVersions(context.Background(), "s_1")
	assertNoError(t, err)
	assertEqual(t, "count", len(versions), 2)
	assertEqual(t, "first semver", versions[0].Semver, "1.0.0")
}
