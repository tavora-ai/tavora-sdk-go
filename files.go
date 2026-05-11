package tavora

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// File is a raw blob in app-scoped Storage. Distinct from
// Document (RAG-indexed view of a file) and Index (RAG container) per
// central-store/docs/RESOURCE_MODEL.md. Bytes-in / bytes-out, with
// sha256-keyed dedup short-circuit on upload.
type File struct {
	ID                string     `json:"id"`
	AppID       string     `json:"app_id"`
	Filename          string     `json:"filename"`
	ContentType       string     `json:"content_type"`
	SizeBytes         int64      `json:"size_bytes"`
	ContentSHA256     string     `json:"content_sha256"`
	OnDiskPath        string     `json:"on_disk_path"`
	CreatedByAPIKeyID *string    `json:"created_by_api_key_id"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	DeletedAt         *time.Time `json:"deleted_at"`
}

// UploadFileInput holds parameters for uploading a file. Either
// FilePath (read from disk) or Content (an io.Reader) must be set.
// Filename is required when using Content.
type UploadFileInput struct {
	FilePath    string
	Content     io.Reader
	Filename    string
	ContentType string
}

// ListFilesInput narrows a list call.
type ListFilesInput struct {
	Limit          int
	Offset         int
	ContentType    string // exact-match filter
	ContentSHA256  string // exact-match filter (use for dedup probes)
	IncludeDeleted bool
}

// ListFilesResult is the paginated list response.
type ListFilesResult struct {
	Data    []File `json:"data"`
	Total   int64  `json:"total"`
	HasMore bool   `json:"has_more"`
}

// UploadFile uploads bytes and returns the File row. If the app
// already has a live file with the same content sha256, the server
// returns the existing row (HTTP 200); otherwise it creates a new
// row (HTTP 201). The caller doesn't need to distinguish — both shapes
// are the same File struct.
func (c *Client) UploadFile(ctx context.Context, input UploadFileInput) (*File, error) {
	if input.FilePath == "" && input.Content == nil {
		return nil, fmt.Errorf("tavora: UploadFile: one of FilePath or Content is required")
	}
	if input.FilePath != "" && input.Content != nil {
		return nil, fmt.Errorf("tavora: UploadFile: FilePath and Content are mutually exclusive")
	}

	filename := input.Filename
	var reader io.Reader = input.Content
	if input.FilePath != "" {
		f, err := os.Open(input.FilePath)
		if err != nil {
			return nil, fmt.Errorf("tavora: opening upload file: %w", err)
		}
		defer f.Close()
		reader = f
		if filename == "" {
			filename = filepath.Base(input.FilePath)
		}
	}
	if filename == "" {
		return nil, fmt.Errorf("tavora: UploadFile: Filename is required when using Content")
	}

	fields := map[string]string{}
	if input.Filename != "" {
		fields["filename"] = filename
	}

	var out File
	if err := c.uploadReader(ctx, "/api/sdk/files", filename, reader, fields, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListFiles returns a paginated list with optional filters.
func (c *Client) ListFiles(ctx context.Context, input ListFilesInput) (*ListFilesResult, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	q := url.Values{}
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(input.Offset))
	if input.ContentType != "" {
		q.Set("content_type", input.ContentType)
	}
	if input.ContentSHA256 != "" {
		q.Set("content_sha256", input.ContentSHA256)
	}
	if input.IncludeDeleted {
		q.Set("include_deleted", "true")
	}
	var out ListFilesResult
	if err := c.get(ctx, "/api/sdk/files?"+q.Encode(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetFile returns a file's metadata by ID.
func (c *Client) GetFile(ctx context.Context, id string) (*File, error) {
	var out File
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/files/%s", id), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetFileContent streams the raw bytes. Caller closes.
func (c *Client) GetFileContent(ctx context.Context, id string) (io.ReadCloser, error) {
	resp, err := c.resty.R().
		SetContext(ctx).
		SetDoNotParseResponse(true).
		Get(fmt.Sprintf("/api/sdk/files/%s/content", id))
	if err != nil {
		return nil, fmt.Errorf("tavora: get file content: %w", err)
	}
	if resp.StatusCode() >= 400 {
		body, _ := io.ReadAll(resp.RawBody())
		_ = resp.RawBody().Close()
		apiErr := parseAPIError(resp.StatusCode(), body)
		if apiErr.Message == "" {
			apiErr.Message = resp.Status()
		}
		return nil, apiErr
	}
	return resp.RawBody(), nil
}

// DeleteFile soft-deletes by default. Use DeleteFileHard for permanent
// removal — fails if any live document references the file (FK ON
// DELETE RESTRICT).
func (c *Client) DeleteFile(ctx context.Context, id string) error {
	return c.delete(ctx, fmt.Sprintf("/api/sdk/files/%s", id))
}

// DeleteFileHard permanently removes a file and its on-disk bytes.
// Returns 5xx if a live document references the file.
func (c *Client) DeleteFileHard(ctx context.Context, id string) error {
	return c.delete(ctx, fmt.Sprintf("/api/sdk/files/%s?hard=true", id))
}
