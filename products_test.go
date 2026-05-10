package tavora

import (
	"context"
	"net/http"
	"testing"
)

func TestGetProduct(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/product", 200, Product{
		ID:   "sp_123",
		Name: "My Product",
		Slug: "my-space",
	})

	space, err := ts.client().GetProduct(context.Background())
	assertNoError(t, err)
	assertEqual(t, "id", space.ID, "sp_123")
	assertEqual(t, "name", space.Name, "My Product")
	assertEqual(t, "slug", space.Slug, "my-space")

	req := ts.lastRequest(t)
	assertEqual(t, "method", req.Method, http.MethodGet)
	assertEqual(t, "path", req.Path, "/api/sdk/product")
	assertEqual(t, "api-key", req.Header.Get("X-API-Key"), "tvr_testkey")
}

func TestGetProduct_Unauthorized(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/product", 401, map[string]string{
		"message": "invalid API key",
	})

	_, err := ts.client().GetProduct(context.Background())
	assertError(t, err)
	if !IsUnauthorized(err) {
		t.Errorf("expected unauthorized error, got %v", err)
	}
}
