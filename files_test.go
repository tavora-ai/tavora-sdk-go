package tavora

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestUploadFile_FromReader(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/files", 201, File{
		ID:            "file_new",
		Filename:      "screenshot.png",
		ContentType:   "image/png",
		SizeBytes:     42,
		ContentSHA256: "abc123",
	})

	got, err := ts.client().UploadFile(context.Background(), UploadFileInput{
		Content:  bytes.NewReader([]byte("PNG bytes go here")),
		Filename: "screenshot.png",
	})
	assertNoError(t, err)
	assertEqual(t, "id", got.ID, "file_new")
	assertEqual(t, "filename", got.Filename, "screenshot.png")
	assertEqual(t, "sha256", got.ContentSHA256, "abc123")

	req := ts.lastRequest(t)
	assertContains(t, req.Body, `name="file"`)
	assertContains(t, req.Body, `name="filename"`)
}

func TestUploadFile_RejectsConflictingInputs(t *testing.T) {
	c := NewClient("http://example.invalid", "tvr_x")
	_, err := c.UploadFile(context.Background(), UploadFileInput{
		FilePath: "/tmp/x",
		Content:  bytes.NewReader([]byte("y")),
		Filename: "x.png",
	})
	assertError(t, err)
}

func TestUploadFile_DedupReturnsExisting(t *testing.T) {
	// Server returns 200 + existing row when the same sha256 is found.
	// SDK should round-trip the same way regardless of status — caller
	// gets the File row either way.
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/files", 200, File{
		ID:            "file_existing",
		ContentSHA256: "deadbeef",
	})
	got, err := ts.client().UploadFile(context.Background(), UploadFileInput{
		Content:  bytes.NewReader([]byte("dup")),
		Filename: "dup.bin",
	})
	assertNoError(t, err)
	assertEqual(t, "id", got.ID, "file_existing")
}

func TestListFiles_FilterByContentType(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/files", 200, ListFilesResult{
		Data: []File{{ID: "f_1", ContentType: "image/png"}},
	})
	_, err := ts.client().ListFiles(context.Background(), ListFilesInput{
		ContentType: "image/png",
		Limit:       10,
	})
	assertNoError(t, err)
	req := ts.lastRequest(t)
	assertContains(t, req.Path, "content_type=image%2Fpng")
	assertContains(t, req.Path, "limit=10")
}

func TestGetFile(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/files/f_1", 200, File{ID: "f_1", Filename: "a.pdf"})
	got, err := ts.client().GetFile(context.Background(), "f_1")
	assertNoError(t, err)
	assertEqual(t, "id", got.ID, "f_1")
}

func TestDeleteFile_SoftByDefault(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodDelete, "/api/sdk/files/f_1", 204, nil)
	err := ts.client().DeleteFile(context.Background(), "f_1")
	assertNoError(t, err)
	req := ts.lastRequest(t)
	if strings.Contains(req.Path, "hard=true") {
		t.Fatalf("default delete must be soft, got %q", req.Path)
	}
}

func TestDeleteFileHard(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodDelete, "/api/sdk/files/f_1", 204, nil)
	err := ts.client().DeleteFileHard(context.Background(), "f_1")
	assertNoError(t, err)
	req := ts.lastRequest(t)
	assertContains(t, req.Path, "hard=true")
}
