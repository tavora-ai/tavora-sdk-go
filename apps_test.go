package tavora

import (
	"context"
	"net/http"
	"testing"
)

func TestGetApp(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/app", 200, App{
		ID:   "sp_123",
		Name: "My App",
		Slug: "my-space",
	})

	space, err := ts.client().GetApp(context.Background())
	assertNoError(t, err)
	assertEqual(t, "id", space.ID, "sp_123")
	assertEqual(t, "name", space.Name, "My App")
	assertEqual(t, "slug", space.Slug, "my-space")

	req := ts.lastRequest(t)
	assertEqual(t, "method", req.Method, http.MethodGet)
	assertEqual(t, "path", req.Path, "/api/sdk/app")
	assertEqual(t, "api-key", req.Header.Get("X-API-Key"), "tvr_testkey")
}

func TestGetApp_Unauthorized(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/app", 401, map[string]string{
		"message": "invalid API key",
	})

	_, err := ts.client().GetApp(context.Background())
	assertError(t, err)
	if !IsUnauthorized(err) {
		t.Errorf("expected unauthorized error, got %v", err)
	}
}
