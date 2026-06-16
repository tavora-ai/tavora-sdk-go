package tavora

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockHeaderServer returns a tiny httptest server that always stamps
// X-Tavora-Mock: true on its responses.
func mockHeaderServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Tavora-Mock", "true")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"00000000-0000-0000-0000-000000000000","slug":"mock","name":"mock","team_id":"","description":"","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`))
	}))
}

func TestSDKRefusesMockByDefault(t *testing.T) {
	t.Setenv("TAVORA_ALLOW_MOCK", "")
	ts := mockHeaderServer(t)
	defer ts.Close()

	c := NewClient(ts.URL, "tvr_key")
	_, err := c.GetProject(context.Background())
	if err == nil {
		t.Fatal("expected ErrMockRefused, got nil")
	}
	if !errors.Is(err, ErrMockRefused) {
		t.Errorf("error = %v, want ErrMockRefused", err)
	}
}

func TestSDKAcceptsMockWhenAllowed(t *testing.T) {
	t.Setenv("TAVORA_ALLOW_MOCK", "1")
	ts := mockHeaderServer(t)
	defer ts.Close()

	c := NewClient(ts.URL, "tvr_key")
	p, err := c.GetProject(context.Background())
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if p.Slug != "mock" {
		t.Errorf("Slug = %q, want %q", p.Slug, "mock")
	}
}

func TestSDKIgnoresMissingMockHeader(t *testing.T) {
	// Real server doesn't set the header; SDK should not interfere.
	t.Setenv("TAVORA_ALLOW_MOCK", "")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"real","slug":"real","name":"","team_id":"","description":"","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "tvr_key")
	p, err := c.GetProject(context.Background())
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if p.Slug != "real" {
		t.Errorf("Slug = %q, want %q", p.Slug, "real")
	}
}
