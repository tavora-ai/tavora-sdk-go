package tavora

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Document represents an uploaded document.
//
// Provenance fields (Name, Version, Metadata, ParentID, CreatedByAPIKeyID,
// DeletedAt) were added by the central-store SDK-validation work. Older
// documents return zero/nil values; new ones round-trip the metadata the
// caller passed to UploadDocument.
type Document struct {
	ID                string          `json:"id"`
	WorkspaceID       string          `json:"workspace_id"`
	StoreID           string          `json:"store_id"`
	Filename          string          `json:"filename"`
	ContentType       string          `json:"content_type"`
	FileSize          int64           `json:"file_size"`
	Status            string          `json:"status"` // pending|processing|ready|stored|error
	ErrorMessage      *string         `json:"error_message"`
	PageCount         *int32          `json:"page_count"`
	ChunkCount        int32           `json:"chunk_count"`
	Name              *string         `json:"name"`
	Version           int32           `json:"version"`
	IsLatest          bool            `json:"is_latest"`
	Metadata          json.RawMessage `json:"metadata"`
	ParentID          *string         `json:"parent_id"`
	CreatedByAPIKeyID *string         `json:"created_by_api_key_id"`
	// ContentSHA256 is the hex-encoded sha256 of the uploaded bytes.
	// Populated by the server on upload; stable across replays. Use
	// ListDocumentsInput.ContentSHA256 or .DuplicateOf to find peers.
	ContentSHA256 *string    `json:"content_sha256"`
	DeletedAt     *time.Time `json:"deleted_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// ListDocumentsInput holds optional parameters for listing documents.
type ListDocumentsInput struct {
	Limit    int               `json:"limit,omitempty"`
	Offset   int               `json:"offset,omitempty"`
	StoreID  string            `json:"store_id,omitempty"`
	Query    string            `json:"q,omitempty"`        // ILIKE filter on filename
	Source   string            `json:"source,omitempty"`   // metadata->>'source' equality
	Metadata map[string]string `json:"metadata,omitempty"` // metadata @> jsonb filter

	// ParentID limits results to artifacts whose parent_id equals this
	// ID. The natural way to fetch the auto-generated markdown sibling
	// of a PDF: pass the PDF's ID here.
	ParentID string `json:"parent_id,omitempty"`

	// DerivedFrom filters on metadata.derived_from. The pipeline stamps
	// "extraction" on auto-generated markdown siblings, so passing
	// "extraction" returns only the extracted-form rows.
	DerivedFrom string `json:"derived_from,omitempty"`

	// ContentSHA256 returns artifacts with this exact content hash.
	ContentSHA256 string `json:"content_sha256,omitempty"`

	// DuplicateOf is sugar over ContentSHA256: the server resolves this
	// document, copies its hash into the filter, and excludes the source
	// itself from the result. Useful for "is this PDF already uploaded?"
	DuplicateOf string `json:"duplicate_of,omitempty"`

	IncludeDeleted bool `json:"include_deleted,omitempty"`
}

// ListDocumentsResult holds a paginated list of documents.
type ListDocumentsResult struct {
	Data    []Document `json:"data"`
	Total   int64      `json:"total"`
	HasMore bool       `json:"has_more"`
}

// SearchInput holds the parameters for a semantic search.
type SearchInput struct {
	Query    string  `json:"query"`
	StoreID  string  `json:"store_id,omitempty"`
	TopK     int32   `json:"top_k,omitempty"`
	MinScore float64 `json:"min_score,omitempty"`

	// ResultType selects the response shape.
	//   "" / "chunk"  — chunk-shaped (default). One row per chunk; use
	//                   for RAG context-building where you want N
	//                   passages across M documents.
	//   "document"    — document-shaped, server-deduped. One row per
	//                   distinct document with the best chunk inlined
	//                   as a preview. Use for "what artifacts are
	//                   about X" queries.
	// Callers fetch the document-shaped response with SearchDocuments.
	ResultType string `json:"result_type,omitempty"`
}

// SearchResult represents a single chunk-shaped search hit. DocumentName +
// DocumentMetadata avoid an N+1 fetch when the caller wants to display
// provenance per hit.
type SearchResult struct {
	ChunkID          string          `json:"chunk_id"`
	DocumentID       string          `json:"document_id"`
	Filename         string          `json:"filename"`
	DocumentName     *string         `json:"document_name"`
	DocumentMetadata json.RawMessage `json:"document_metadata"`
	Content          string          `json:"content"`
	Score            float64         `json:"score"`
	ChunkIndex       int32           `json:"chunk_index"`
	Metadata         json.RawMessage `json:"metadata"`
}

// DocumentSearchResult is the document-mode equivalent of SearchResult —
// one row per distinct document with the best chunk inlined as a
// preview. Returned by SearchDocuments.
type DocumentSearchResult struct {
	DocumentID       string          `json:"document_id"`
	StoreID          string          `json:"store_id"`
	Filename         string          `json:"filename"`
	DocumentName     *string         `json:"document_name"`
	DocumentMetadata json.RawMessage `json:"document_metadata"`
	ParentID         *string         `json:"parent_id"`
	ContentSHA256    *string         `json:"content_sha256"`
	Score            float64         `json:"score"`
	BestChunk        struct {
		ChunkID    string `json:"chunk_id"`
		ChunkIndex int32  `json:"chunk_index"`
		Preview    string `json:"preview"`
	} `json:"best_chunk"`
}

// UploadDocumentInput holds parameters for uploading a document.
//
// One of FilePath or Content must be set. When Content is used, Filename
// is required (it determines the on-server filename and the extension
// used for indexability detection).
type UploadDocumentInput struct {
	StoreID  string
	FilePath string    // mutually exclusive with Content
	Content  io.Reader // mutually exclusive with FilePath; Filename required
	Filename string    // overrides the basename of FilePath; required with Content

	// Provenance — round-tripped through document metadata.
	Name     string            // optional logical name; enables version-on-rewrite
	Source   string            // shorthand for Metadata["source"]
	Task     string            // shorthand for Metadata["task"]
	Type     string            // shorthand for Metadata["type"]
	Tags     map[string]string // merged into Metadata as flat keys
	Metadata map[string]string // arbitrary metadata; merged with Source/Task/Type/Tags
	ParentID string            // optional ULID/UUID of the parent artifact

	// Optimistic concurrency. Server returns 409 if the current latest
	// (StoreID, Name) version doesn't equal IfVersion.
	IfVersion *int32
}

// UploadDocument uploads a file (from disk or an io.Reader) and triggers
// async processing on the server. Indexable types (.pdf, .md, .txt, .csv,
// .html, .docx, .xlsx, images) go through extract+chunk+embed; other
// types are stored opaque (status="stored") so they round-trip but aren't
// semantically searchable.
func (c *Client) UploadDocument(ctx context.Context, input UploadDocumentInput) (*Document, error) {
	if input.StoreID == "" {
		return nil, fmt.Errorf("tavora: UploadDocument: StoreID is required")
	}
	if input.FilePath == "" && input.Content == nil {
		return nil, fmt.Errorf("tavora: UploadDocument: one of FilePath or Content is required")
	}
	if input.FilePath != "" && input.Content != nil {
		return nil, fmt.Errorf("tavora: UploadDocument: FilePath and Content are mutually exclusive")
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
		return nil, fmt.Errorf("tavora: UploadDocument: Filename is required when using Content")
	}

	fields := map[string]string{
		"store_id": input.StoreID,
	}
	if input.Name != "" {
		fields["name"] = input.Name
	}
	if input.ParentID != "" {
		fields["parent_id"] = input.ParentID
	}
	if input.IfVersion != nil {
		fields["if_version"] = strconv.Itoa(int(*input.IfVersion))
	}

	// Merge the convenience shorthands into the metadata map so the
	// server sees a single JSON blob.
	meta := map[string]string{}
	for k, v := range input.Metadata {
		meta[k] = v
	}
	if input.Source != "" {
		meta["source"] = input.Source
	}
	if input.Task != "" {
		meta["task"] = input.Task
	}
	if input.Type != "" {
		meta["type"] = input.Type
	}
	for k, v := range input.Tags {
		meta[k] = v
	}
	if len(meta) > 0 {
		b, err := json.Marshal(meta)
		if err != nil {
			return nil, fmt.Errorf("tavora: marshaling metadata: %w", err)
		}
		fields["metadata"] = string(b)
	}

	path := fmt.Sprintf("/api/sdk/stores/%s/documents", input.StoreID)
	var doc Document
	if err := c.uploadReader(ctx, path, filename, reader, fields, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

func buildListQuery(base string, in ListDocumentsInput) string {
	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}
	q := url.Values{}
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(in.Offset))
	if in.Query != "" {
		q.Set("q", in.Query)
	}
	if in.Source != "" {
		q.Set("source", in.Source)
	}
	if in.IncludeDeleted {
		q.Set("include_deleted", "true")
	}
	if in.ParentID != "" {
		q.Set("parent_id", in.ParentID)
	}
	if in.DerivedFrom != "" {
		q.Set("derived_from", in.DerivedFrom)
	}
	if in.ContentSHA256 != "" {
		q.Set("content_sha256", in.ContentSHA256)
	}
	if in.DuplicateOf != "" {
		q.Set("duplicate_of", in.DuplicateOf)
	}
	for k, v := range in.Metadata {
		q.Set("metadata."+k, v)
	}
	return base + "?" + q.Encode()
}

// ListDocuments returns a paginated list of documents.
func (c *Client) ListDocuments(ctx context.Context, input ListDocumentsInput) (*ListDocumentsResult, error) {
	var base string
	if input.StoreID != "" {
		base = fmt.Sprintf("/api/sdk/stores/%s/documents", input.StoreID)
	} else {
		base = "/api/sdk/documents"
	}

	var result ListDocumentsResult
	if err := c.get(ctx, buildListQuery(base, input), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetDocument returns a single document by ID. Soft-deleted documents
// return 404 unless GetDocumentOptions.IncludeDeleted is set.
func (c *Client) GetDocument(ctx context.Context, id string) (*Document, error) {
	var doc Document
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/documents/%s", id), &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

// GetDocumentByNameInput pins a (store, name) lookup. Version=0 means
// "latest non-deleted version" (the common path); a positive Version
// fetches that exact historical version.
type GetDocumentByNameInput struct {
	StoreID string
	Name    string
	Version int32
}

// GetDocumentByName resolves the latest non-deleted version of (store,
// name), or a specific version when Version is set. This is the
// agent-facing addressing primitive — "give me the current plan" —
// without the caller tracking IDs.
func (c *Client) GetDocumentByName(ctx context.Context, input GetDocumentByNameInput) (*Document, error) {
	if input.StoreID == "" || input.Name == "" {
		return nil, fmt.Errorf("tavora: GetDocumentByName: StoreID and Name are required")
	}
	path := fmt.Sprintf("/api/sdk/stores/%s/documents/by-name/%s", input.StoreID, url.PathEscape(input.Name))
	if input.Version > 0 {
		path += fmt.Sprintf("?version=%d", input.Version)
	}
	var doc Document
	if err := c.get(ctx, path, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

// ListDocumentVersions returns every version of (store, name), newest
// first, including soft-deleted ones — the artifact history.
func (c *Client) ListDocumentVersions(ctx context.Context, storeID, name string) ([]Document, error) {
	path := fmt.Sprintf("/api/sdk/stores/%s/documents/by-name/%s/versions", storeID, url.PathEscape(name))
	var resp struct {
		Versions []Document `json:"versions"`
	}
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return resp.Versions, nil
}

// DeleteDocument soft-deletes a document by ID (default). Use
// DeleteDocumentHard for permanent deletion.
func (c *Client) DeleteDocument(ctx context.Context, id string) error {
	return c.delete(ctx, fmt.Sprintf("/api/sdk/documents/%s", id))
}

// DeleteDocumentHard permanently removes a document and its on-disk file.
// Use sparingly — soft delete is the default for a reason.
func (c *Client) DeleteDocumentHard(ctx context.Context, id string) error {
	return c.delete(ctx, fmt.Sprintf("/api/sdk/documents/%s?hard=true", id))
}

// Search performs a chunk-shaped semantic search. If StoreID is set,
// searches within that store; otherwise searches across all stores.
// For document-shaped results use SearchDocuments.
func (c *Client) Search(ctx context.Context, input SearchInput) ([]SearchResult, error) {
	// Force chunk shape — SearchDocuments is the dedicated method for
	// the document shape, and silently swapping shapes here would
	// surprise callers who already typed against SearchResult.
	input.ResultType = "chunk"
	var resp struct {
		Results []SearchResult `json:"results"`
	}
	if err := c.post(ctx, searchPath(input.StoreID), input, &resp); err != nil {
		return nil, err
	}
	return resp.Results, nil
}

// SearchDocuments runs the same vector query as Search but returns
// distinct documents ranked by their best chunk's score, with that
// chunk inlined as a preview. Server-side aggregation removes the N+1
// dedup work agents would otherwise do client-side.
func (c *Client) SearchDocuments(ctx context.Context, input SearchInput) ([]DocumentSearchResult, error) {
	input.ResultType = "document"
	var resp struct {
		Results []DocumentSearchResult `json:"results"`
	}
	if err := c.post(ctx, searchPath(input.StoreID), input, &resp); err != nil {
		return nil, err
	}
	return resp.Results, nil
}

func searchPath(storeID string) string {
	if storeID != "" {
		return fmt.Sprintf("/api/sdk/stores/%s/search", storeID)
	}
	return "/api/sdk/search"
}
