package tavora

import (
	"context"
	"net/http"
	"testing"
)

func TestGetWorkspace(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/space", 200, Workspace{
		ID:   "sp_123",
		Name: "My Workspace",
		Slug: "my-space",
	})

	space, err := ts.client().GetWorkspace(context.Background())
	assertNoError(t, err)
	assertEqual(t, "id", space.ID, "sp_123")
	assertEqual(t, "name", space.Name, "My Workspace")
	assertEqual(t, "slug", space.Slug, "my-space")

	req := ts.lastRequest(t)
	assertEqual(t, "method", req.Method, http.MethodGet)
	assertEqual(t, "path", req.Path, "/api/sdk/space")
	assertEqual(t, "api-key", req.Header.Get("X-API-Key"), "tvr_testkey")
}

func TestGetWorkspace_Unauthorized(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/space", 401, map[string]string{
		"message": "invalid API key",
	})

	_, err := ts.client().GetWorkspace(context.Background())
	assertError(t, err)
	if !IsUnauthorized(err) {
		t.Errorf("expected unauthorized error, got %v", err)
	}
}
