package tavora

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestListCollections(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/collections", 200, map[string]any{
		"collections": []Collection{
			{Name: "users", Count: 12},
			{Name: "leads", Count: 3},
		},
	})

	colls, err := ts.client().ListCollections(context.Background())
	assertNoError(t, err)
	assertEqual(t, "count", len(colls), 2)
	assertEqual(t, "first name", colls[0].Name, "users")
	assertEqual(t, "first count", colls[0].Count, int64(12))
}

func TestCreateCollection(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/collections", 201, map[string]any{
		"collection": Collection{Name: "leads", Count: 0},
	})

	coll, err := ts.client().CreateCollection(context.Background(), "leads")
	assertNoError(t, err)
	assertEqual(t, "name", coll.Name, "leads")

	req := ts.lastRequest(t)
	assertEqual(t, "method", req.Method, http.MethodPost)
	var body map[string]string
	json.Unmarshal([]byte(req.Body), &body)
	assertEqual(t, "name body", body["name"], "leads")
}

func TestDropCollection(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodDelete, "/api/sdk/collections/leads", 204, nil)

	err := ts.client().DropCollection(context.Background(), "leads")
	assertNoError(t, err)

	req := ts.lastRequest(t)
	assertEqual(t, "method", req.Method, http.MethodDelete)
	assertEqual(t, "path", req.Path, "/api/sdk/collections/leads")
}

func TestInsertCollectionDocument(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/collections/users/documents", 201, map[string]any{
		"id": 42,
	})

	id, err := ts.client().InsertCollectionDocument(context.Background(), "users", CollectionDocument{
		"name": "Alice", "age": 30,
	})
	assertNoError(t, err)
	assertEqual(t, "id", id, int64(42))

	req := ts.lastRequest(t)
	var body struct {
		Document map[string]any `json:"document"`
	}
	json.Unmarshal([]byte(req.Body), &body)
	assertEqual(t, "name", body.Document["name"], "Alice")
}

func TestInsertCollectionDocuments(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/collections/users/documents", 201, map[string]any{
		"ids": []int64{1, 2, 3},
	})

	ids, err := ts.client().InsertCollectionDocuments(context.Background(), "users", []CollectionDocument{
		{"name": "Alice"}, {"name": "Bob"}, {"name": "Carol"},
	})
	assertNoError(t, err)
	assertEqual(t, "count", len(ids), 3)

	req := ts.lastRequest(t)
	var body struct {
		Documents []map[string]any `json:"documents"`
	}
	json.Unmarshal([]byte(req.Body), &body)
	assertEqual(t, "doc count", len(body.Documents), 3)
}

func TestFindCollectionDocuments(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/collections/users/find", 200, map[string]any{
		"documents": []CollectionDocument{
			{"_id": 1, "name": "Alice", "age": 30},
		},
	})

	docs, err := ts.client().FindCollectionDocuments(context.Background(), "users", FindCollectionInput{
		Filter: map[string]any{"age": map[string]any{"$gte": 30}},
		Sort:   "-age",
		Limit:  10,
	})
	assertNoError(t, err)
	assertEqual(t, "count", len(docs), 1)
	assertEqual(t, "name", docs[0]["name"], "Alice")

	req := ts.lastRequest(t)
	assertEqual(t, "path", req.Path, "/api/sdk/collections/users/find")
	var body FindCollectionInput
	json.Unmarshal([]byte(req.Body), &body)
	assertEqual(t, "sort", body.Sort, "-age")
	assertEqual(t, "limit", body.Limit, 10)
}

func TestUpdateCollectionDocuments(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/collections/users/update", 200, map[string]any{
		"updated": 2,
	})

	n, err := ts.client().UpdateCollectionDocuments(context.Background(), "users", UpdateCollectionInput{
		Filter:  map[string]any{"role": "designer"},
		Updates: map[string]any{"role": "lead designer"},
	})
	assertNoError(t, err)
	assertEqual(t, "updated", n, 2)
}

func TestRemoveCollectionDocuments(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/collections/users/remove", 200, map[string]any{
		"removed": 1,
	})

	n, err := ts.client().RemoveCollectionDocuments(context.Background(), "users", RemoveCollectionInput{
		Filter: map[string]any{"name": "Alice"},
	})
	assertNoError(t, err)
	assertEqual(t, "removed", n, 1)
}
