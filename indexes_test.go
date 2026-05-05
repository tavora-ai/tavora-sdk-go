package tavora

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestListIndexes(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/indexes", 200, map[string]interface{}{
		"indexes": []Index{
			{ID: "st_1", Name: "Docs"},
			{ID: "st_2", Name: "Images"},
		},
	})

	stores, err := ts.client().ListIndexes(context.Background())
	assertNoError(t, err)
	assertEqual(t, "count", len(stores), 2)
	assertEqual(t, "first id", stores[0].ID, "st_1")
	assertEqual(t, "second name", stores[1].Name, "Images")
}

func TestGetIndex(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/indexes/st_1", 200, map[string]interface{}{
		"index": Index{ID: "st_1", Name: "Docs"},
	})

	store, err := ts.client().GetIndex(context.Background(), "st_1")
	assertNoError(t, err)
	assertEqual(t, "id", store.ID, "st_1")
	assertEqual(t, "name", store.Name, "Docs")

	req := ts.lastRequest(t)
	assertEqual(t, "path", req.Path, "/api/sdk/indexes/st_1")
}

func TestGetIndex_NotFound(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/indexes/st_missing", 404, map[string]string{
		"message": "store not found",
	})

	_, err := ts.client().GetIndex(context.Background(), "st_missing")
	assertError(t, err)
	if !IsNotFound(err) {
		t.Errorf("expected not found error, got %v", err)
	}
}

func TestCreateIndex(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/indexes", 201, Index{
		ID:   "st_new",
		Name: "New Store",
	})

	store, err := ts.client().CreateIndex(context.Background(), CreateIndexInput{
		Name:        "New Store",
		Description: "A test store",
	})
	assertNoError(t, err)
	assertEqual(t, "id", store.ID, "st_new")

	req := ts.lastRequest(t)
	assertEqual(t, "method", req.Method, http.MethodPost)
	var body CreateIndexInput
	json.Unmarshal([]byte(req.Body), &body)
	assertEqual(t, "name", body.Name, "New Store")
	assertEqual(t, "description", body.Description, "A test store")
}

func TestUpdateIndex(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPatch, "/api/sdk/indexes/st_1", 200, Index{
		ID:   "st_1",
		Name: "Updated",
	})

	store, err := ts.client().UpdateIndex(context.Background(), "st_1", UpdateIndexInput{
		Name: "Updated",
	})
	assertNoError(t, err)
	assertEqual(t, "name", store.Name, "Updated")

	req := ts.lastRequest(t)
	assertEqual(t, "method", req.Method, http.MethodPatch)
	assertEqual(t, "path", req.Path, "/api/sdk/indexes/st_1")
}

func TestDeleteIndex(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodDelete, "/api/sdk/indexes/st_1", 204, nil)

	err := ts.client().DeleteIndex(context.Background(), "st_1")
	assertNoError(t, err)

	req := ts.lastRequest(t)
	assertEqual(t, "method", req.Method, http.MethodDelete)
	assertEqual(t, "path", req.Path, "/api/sdk/indexes/st_1")
}
