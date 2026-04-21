package tavora

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestListStores(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/stores", 200, map[string]interface{}{
		"stores": []Store{
			{ID: "st_1", Name: "Docs"},
			{ID: "st_2", Name: "Images"},
		},
	})

	stores, err := ts.client().ListStores(context.Background())
	assertNoError(t, err)
	assertEqual(t, "count", len(stores), 2)
	assertEqual(t, "first id", stores[0].ID, "st_1")
	assertEqual(t, "second name", stores[1].Name, "Images")
}

func TestGetStore(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/stores/st_1", 200, map[string]interface{}{
		"store": Store{ID: "st_1", Name: "Docs"},
	})

	store, err := ts.client().GetStore(context.Background(), "st_1")
	assertNoError(t, err)
	assertEqual(t, "id", store.ID, "st_1")
	assertEqual(t, "name", store.Name, "Docs")

	req := ts.lastRequest(t)
	assertEqual(t, "path", req.Path, "/api/sdk/stores/st_1")
}

func TestGetStore_NotFound(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/stores/st_missing", 404, map[string]string{
		"message": "store not found",
	})

	_, err := ts.client().GetStore(context.Background(), "st_missing")
	assertError(t, err)
	if !IsNotFound(err) {
		t.Errorf("expected not found error, got %v", err)
	}
}

func TestCreateStore(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/stores", 201, Store{
		ID:   "st_new",
		Name: "New Store",
	})

	store, err := ts.client().CreateStore(context.Background(), CreateStoreInput{
		Name:        "New Store",
		Description: "A test store",
	})
	assertNoError(t, err)
	assertEqual(t, "id", store.ID, "st_new")

	req := ts.lastRequest(t)
	assertEqual(t, "method", req.Method, http.MethodPost)
	var body CreateStoreInput
	json.Unmarshal([]byte(req.Body), &body)
	assertEqual(t, "name", body.Name, "New Store")
	assertEqual(t, "description", body.Description, "A test store")
}

func TestUpdateStore(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPatch, "/api/sdk/stores/st_1", 200, Store{
		ID:   "st_1",
		Name: "Updated",
	})

	store, err := ts.client().UpdateStore(context.Background(), "st_1", UpdateStoreInput{
		Name: "Updated",
	})
	assertNoError(t, err)
	assertEqual(t, "name", store.Name, "Updated")

	req := ts.lastRequest(t)
	assertEqual(t, "method", req.Method, http.MethodPatch)
	assertEqual(t, "path", req.Path, "/api/sdk/stores/st_1")
}

func TestDeleteStore(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodDelete, "/api/sdk/stores/st_1", 204, nil)

	err := ts.client().DeleteStore(context.Background(), "st_1")
	assertNoError(t, err)

	req := ts.lastRequest(t)
	assertEqual(t, "method", req.Method, http.MethodDelete)
	assertEqual(t, "path", req.Path, "/api/sdk/stores/st_1")
}
