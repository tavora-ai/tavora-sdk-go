package tavora

import (
	"context"
	"net/http"
	"testing"
)

func TestGetProject(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/project", 200, Project{
		ID:   "sp_123",
		Name: "My Project",
		Slug: "my-space",
	})

	space, err := ts.client().GetProject(context.Background())
	assertNoError(t, err)
	assertEqual(t, "id", space.ID, "sp_123")
	assertEqual(t, "name", space.Name, "My Project")
	assertEqual(t, "slug", space.Slug, "my-space")

	req := ts.lastRequest(t)
	assertEqual(t, "method", req.Method, http.MethodGet)
	assertEqual(t, "path", req.Path, "/api/sdk/project")
	assertEqual(t, "api-key", req.Header.Get("X-API-Key"), "tvr_testkey")
}

func TestGetProject_Unauthorized(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/project", 401, map[string]string{
		"message": "invalid API key",
	})

	_, err := ts.client().GetProject(context.Background())
	assertError(t, err)
	if !IsUnauthorized(err) {
		t.Errorf("expected unauthorized error, got %v", err)
	}
}
