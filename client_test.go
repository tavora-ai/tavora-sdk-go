package tavora

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient("https://api.tavora.ai", "tvr_test123")
	if c.resty == nil {
		t.Fatal("expected resty client to be initialized")
	}
}

func TestClient_APIKeyHeader(t *testing.T) {
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tvr_mykey")
	var result map[string]string
	err := c.get(context.Background(), "/test", &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotKey != "tvr_mykey" {
		t.Errorf("expected X-API-Key tvr_mykey, got %s", gotKey)
	}
}

func TestClient_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"message": "invalid API key"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tvr_bad")
	err := c.get(context.Background(), "/test", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsUnauthorized(err) {
		t.Errorf("expected unauthorized error, got %v", err)
	}
}

func TestClient_Post(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": "123"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tvr_key")
	var result map[string]string
	err := c.post(context.Background(), "/test", map[string]string{"name": "test"}, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["id"] != "123" {
		t.Errorf("expected id 123, got %s", result["id"])
	}
}
