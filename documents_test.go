package tavora

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
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
	if !strings.HasPrefix(req.Path, "/api/sdk/documents?") {
		t.Fatalf("path: got %q, want prefix /api/sdk/documents?", req.Path)
	}
	assertContains(t, req.Path, "limit=50")
	assertContains(t, req.Path, "offset=0")
}

func TestListDocuments_WithFilters(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/stores/st_1/documents", 200, ListDocumentsResult{
		Data: []Document{{ID: "doc_1"}},
	})

	_, err := ts.client().ListDocuments(context.Background(), ListDocumentsInput{
		StoreID:        "st_1",
		Limit:          10,
		Offset:         5,
		Query:          "readme",
		Source:         "claude-code",
		Metadata:       map[string]string{"task": "refactor"},
		IncludeDeleted: true,
	})
	assertNoError(t, err)

	req := ts.lastRequest(t)
	assertContains(t, req.Path, "limit=10")
	assertContains(t, req.Path, "offset=5")
	assertContains(t, req.Path, "q=readme")
	assertContains(t, req.Path, "source=claude-code")
	assertContains(t, req.Path, "metadata.task=refactor")
	assertContains(t, req.Path, "include_deleted=true")
}

func TestGetDocument(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/documents/doc_1", 200, Document{
		ID:       "doc_1",
		Filename: "readme.md",
		Status:   "ready",
		Version:  1,
	})

	doc, err := ts.client().GetDocument(context.Background(), "doc_1")
	assertNoError(t, err)
	assertEqual(t, "id", doc.ID, "doc_1")
	assertEqual(t, "status", doc.Status, "ready")
	assertEqual(t, "version", doc.Version, int32(1))
}

func TestGetDocumentByName_Latest(t *testing.T) {
	ts := newTestServer(t)
	name := "current_plan"
	ts.on(http.MethodGet, "/api/sdk/stores/st_1/documents/by-name/current_plan", 200, Document{
		ID:      "doc_42",
		Name:    &name,
		Version: 3,
	})

	doc, err := ts.client().GetDocumentByName(context.Background(), GetDocumentByNameInput{
		StoreID: "st_1",
		Name:    "current_plan",
	})
	assertNoError(t, err)
	assertEqual(t, "id", doc.ID, "doc_42")
	assertEqual(t, "version", doc.Version, int32(3))
}

func TestGetDocumentByName_PinnedVersion(t *testing.T) {
	ts := newTestServer(t)
	name := "current_plan"
	ts.on(http.MethodGet, "/api/sdk/stores/st_1/documents/by-name/current_plan", 200, Document{
		ID: "doc_42", Name: &name, Version: 2,
	})

	_, err := ts.client().GetDocumentByName(context.Background(), GetDocumentByNameInput{
		StoreID: "st_1",
		Name:    "current_plan",
		Version: 2,
	})
	assertNoError(t, err)

	req := ts.lastRequest(t)
	assertContains(t, req.Path, "version=2")
}

func TestListDocumentVersions(t *testing.T) {
	ts := newTestServer(t)
	name := "current_plan"
	ts.on(http.MethodGet, "/api/sdk/stores/st_1/documents/by-name/current_plan/versions", 200, map[string]any{
		"versions": []Document{
			{ID: "doc_43", Name: &name, Version: 2, IsLatest: true},
			{ID: "doc_42", Name: &name, Version: 1, IsLatest: false},
		},
	})

	versions, err := ts.client().ListDocumentVersions(context.Background(), "st_1", "current_plan")
	assertNoError(t, err)
	assertEqual(t, "count", len(versions), 2)
	assertEqual(t, "latest version", versions[0].Version, int32(2))
}

func TestDeleteDocument_SoftByDefault(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodDelete, "/api/sdk/documents/doc_1", 204, nil)

	err := ts.client().DeleteDocument(context.Background(), "doc_1")
	assertNoError(t, err)

	req := ts.lastRequest(t)
	if strings.Contains(req.Path, "hard=true") {
		t.Fatalf("default delete must be soft, got %q", req.Path)
	}
}

func TestDeleteDocumentHard(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodDelete, "/api/sdk/documents/doc_1", 204, nil)

	err := ts.client().DeleteDocumentHard(context.Background(), "doc_1")
	assertNoError(t, err)

	req := ts.lastRequest(t)
	assertContains(t, req.Path, "hard=true")
}

func TestUploadDocument_FromReader(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/stores/st_1/documents", 201, Document{
		ID: "doc_new", StoreID: "st_1", Filename: "plan.md",
	})

	doc, err := ts.client().UploadDocument(context.Background(), UploadDocumentInput{
		StoreID:  "st_1",
		Content:  bytes.NewReader([]byte("# plan\n")),
		Filename: "plan.md",
		Name:     "current_plan",
		Source:   "claude-code",
		Task:     "refactor-auth",
		Metadata: map[string]string{"branch": "main"},
	})
	assertNoError(t, err)
	assertEqual(t, "id", doc.ID, "doc_new")

	req := ts.lastRequest(t)
	assertContains(t, req.Body, `name="name"`)
	assertContains(t, req.Body, "current_plan")
	assertContains(t, req.Body, `name="metadata"`)
	// Metadata is JSON; verify the shorthand fields landed in the JSON blob.
	assertContains(t, req.Body, `"source":"claude-code"`)
	assertContains(t, req.Body, `"task":"refactor-auth"`)
	assertContains(t, req.Body, `"branch":"main"`)
}

func TestUploadDocument_RejectsConflictingInputs(t *testing.T) {
	ts := newTestServer(t)
	_ = ts
	c := NewClient("http://example.invalid", "tvr_x")
	_, err := c.UploadDocument(context.Background(), UploadDocumentInput{
		StoreID: "st_1",
		// Both FilePath and Content set — should error before any HTTP.
		FilePath: "/tmp/x",
		Content:  bytes.NewReader([]byte("y")),
		Filename: "x.md",
	})
	assertError(t, err)
}

func TestUploadDocument_IfVersion(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/stores/st_1/documents", 201, Document{ID: "doc_new"})

	v := int32(2)
	_, err := ts.client().UploadDocument(context.Background(), UploadDocumentInput{
		StoreID:   "st_1",
		Content:   bytes.NewReader([]byte("v3")),
		Filename:  "plan.md",
		Name:      "current_plan",
		IfVersion: &v,
	})
	assertNoError(t, err)

	req := ts.lastRequest(t)
	assertContains(t, req.Body, `name="if_version"`)
	assertContains(t, req.Body, "2")
}

func TestSearch(t *testing.T) {
	ts := newTestServer(t)
	docName := "plan"
	ts.on(http.MethodPost, "/api/sdk/search", 200, map[string]interface{}{
		"results": []SearchResult{{
			ChunkID:          "ch_1",
			Content:          "hello world",
			Score:            0.95,
			DocumentName:     &docName,
			DocumentMetadata: json.RawMessage(`{"source":"claude-code"}`),
		}},
	})

	results, err := ts.client().Search(context.Background(), SearchInput{
		Query: "hello",
		TopK:  5,
	})
	assertNoError(t, err)
	assertEqual(t, "count", len(results), 1)
	assertEqual(t, "content", results[0].Content, "hello world")
	if results[0].DocumentName == nil || *results[0].DocumentName != "plan" {
		t.Fatalf("DocumentName: got %v, want plan", results[0].DocumentName)
	}
	if !bytes.Contains(results[0].DocumentMetadata, []byte("claude-code")) {
		t.Fatalf("DocumentMetadata: got %s, want to contain claude-code", results[0].DocumentMetadata)
	}

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

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected %q to contain %q", haystack, needle)
	}
}
