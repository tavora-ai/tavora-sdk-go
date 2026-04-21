package tavora

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestListDocuments(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/documents", 200, ListDocumentsResult{
		Data:    []Document{{ID: "doc_1", Filename: "readme.md"}},
		Total:   1,
		HasMore: false,
	})

	result, err := ts.client().ListDocuments(context.Background(), ListDocumentsInput{})
	assertNoError(t, err)
	assertEqual(t, "count", len(result.Data), 1)
	assertEqual(t, "filename", result.Data[0].Filename, "readme.md")
	assertEqual(t, "total", result.Total, int64(1))

	req := ts.lastRequest(t)
	assertEqual(t, "path", req.Path, "/api/sdk/documents?limit=50&offset=0")
}

func TestListDocuments_WithStore(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/stores/st_1/documents", 200, ListDocumentsResult{
		Data: []Document{{ID: "doc_1"}},
	})

	_, err := ts.client().ListDocuments(context.Background(), ListDocumentsInput{
		StoreID: "st_1",
		Limit:   10,
		Offset:  5,
	})
	assertNoError(t, err)

	req := ts.lastRequest(t)
	assertEqual(t, "path", req.Path, "/api/sdk/stores/st_1/documents?limit=10&offset=5")
}

func TestGetDocument(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/documents/doc_1", 200, Document{
		ID:       "doc_1",
		Filename: "readme.md",
		Status:   "ready",
	})

	doc, err := ts.client().GetDocument(context.Background(), "doc_1")
	assertNoError(t, err)
	assertEqual(t, "id", doc.ID, "doc_1")
	assertEqual(t, "status", doc.Status, "ready")
}

func TestDeleteDocument(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodDelete, "/api/sdk/documents/doc_1", 204, nil)

	err := ts.client().DeleteDocument(context.Background(), "doc_1")
	assertNoError(t, err)

	req := ts.lastRequest(t)
	assertEqual(t, "path", req.Path, "/api/sdk/documents/doc_1")
}

func TestSearch(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/search", 200, map[string]interface{}{
		"results": []SearchResult{
			{ChunkID: "ch_1", Content: "hello world", Score: 0.95},
		},
	})

	results, err := ts.client().Search(context.Background(), SearchInput{
		Query: "hello",
		TopK:  5,
	})
	assertNoError(t, err)
	assertEqual(t, "count", len(results), 1)
	assertEqual(t, "content", results[0].Content, "hello world")

	req := ts.lastRequest(t)
	assertEqual(t, "method", req.Method, http.MethodPost)
	var body SearchInput
	json.Unmarshal([]byte(req.Body), &body)
	assertEqual(t, "query", body.Query, "hello")
	assertEqual(t, "top_k", body.TopK, int32(5))
}

func TestSearch_WithStore(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/stores/st_1/search", 200, map[string]interface{}{
		"results": []SearchResult{},
	})

	_, err := ts.client().Search(context.Background(), SearchInput{
		Query:   "test",
		StoreID: "st_1",
	})
	assertNoError(t, err)

	req := ts.lastRequest(t)
	assertEqual(t, "path", req.Path, "/api/sdk/stores/st_1/search")
}
