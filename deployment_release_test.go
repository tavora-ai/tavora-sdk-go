package tavora

import (
	"context"
	"net/http"
	"testing"
)

func TestListReleases(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/projects/demo/deployments/prod-slug/releases", 200, map[string]interface{}{
		"releases": []ReleaseRecord{
			{ID: "r2", DeploymentID: "d1", Seq: 2, Action: "rollback", SourceReleaseID: "r1", AgentCount: 2},
			{ID: "r1", DeploymentID: "d1", Seq: 1, Action: "promote", AgentCount: 2},
		},
	})

	releases, err := ts.client().ListReleases(context.Background(), "demo", "prod-slug")
	assertNoError(t, err)
	assertEqual(t, "count", len(releases), 2)
	assertEqual(t, "newest seq", releases[0].Seq, int64(2))
	assertEqual(t, "newest action", releases[0].Action, "rollback")
	assertEqual(t, "source", releases[0].SourceReleaseID, "r1")

	req := ts.lastRequest(t)
	assertEqual(t, "method", req.Method, http.MethodGet)
	assertEqual(t, "path", req.Path, "/api/sdk/projects/demo/deployments/prod-slug/releases")
}

func TestRollback(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/projects/demo/deployments/prod-slug/rollback", 200, Release{
		Deployment: ProjectEnvironment{ID: "d1", Kind: "prod", Slug: "prod-slug"},
		Pins:       []VersionPin{{AgentID: "agt-A", VersionNumber: 1}},
		ReleaseID:  "r3",
		Seq:        3,
		Action:     "rollback",
	})

	rel, err := ts.client().Rollback(context.Background(), "demo", "prod-slug", "r1")
	assertNoError(t, err)
	assertEqual(t, "action", rel.Action, "rollback")
	assertEqual(t, "seq", rel.Seq, int64(3))
	assertEqual(t, "pin version", rel.Pins[0].VersionNumber, int64(1))

	req := ts.lastRequest(t)
	assertEqual(t, "method", req.Method, http.MethodPost)
	assertEqual(t, "path", req.Path, "/api/sdk/projects/demo/deployments/prod-slug/rollback")
	assertContains(t, req.Body, `"release_id":"r1"`)
}
