package tavora

import (
	"context"
	"fmt"
	"time"
)

// Document represents an uploaded document.
type Document struct {
	ID           string    `json:"id"`
	WorkspaceID      string    `json:"workspace_id"`
	StoreID      string    `json:"store_id"`
	Filename     string    `json:"filename"`
	ContentType  string    `json:"content_type"`
	FileSize     int64     `json:"file_size"`
	Status       string    `json:"status"`
	ErrorMessage *string   `json:"error_message"`
	PageCount    *int32    `json:"page_count"`
	ChunkCount   int32     `json:"chunk_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ListDocumentsInput holds optional parameters for listing documents.
type ListDocumentsInput struct {
	Limit   int    `json:"limit,omitempty"`
	Offset  int    `json:"offset,omitempty"`
	StoreID string `json:"store_id,omitempty"`
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
}

// SearchResult represents a single search hit.
type SearchResult struct {
	ChunkID    string  `json:"chunk_id"`
	DocumentID string  `json:"document_id"`
	Filename   string  `json:"filename"`
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
	ChunkIndex int32   `json:"chunk_index"`
}

// UploadDocumentInput holds parameters for uploading a document.
type UploadDocumentInput struct {
	// FilePath is the local path to the file to upload.
	FilePath string
	// StoreID assigns the document to a store.
	StoreID string
}

// UploadDocument uploads a file and triggers async processing.
// Supported file types: .pdf, .md, .txt, .csv
func (c *Client) UploadDocument(ctx context.Context, input UploadDocumentInput) (*Document, error) {
	fields := map[string]string{}
	if input.StoreID != "" {
		fields["store_id"] = input.StoreID
	}
	path := fmt.Sprintf("/api/sdk/stores/%s/documents", input.StoreID)
	var doc Document
	if err := c.upload(ctx, path, input.FilePath, fields, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

// ListDocuments returns a paginated list of documents.
func (c *Client) ListDocuments(ctx context.Context, input ListDocumentsInput) (*ListDocumentsResult, error) {
	var path string
	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	if input.StoreID != "" {
		path = fmt.Sprintf("/api/sdk/stores/%s/documents?limit=%d&offset=%d", input.StoreID, limit, input.Offset)
	} else {
		path = fmt.Sprintf("/api/sdk/documents?limit=%d&offset=%d", limit, input.Offset)
	}

	var result ListDocumentsResult
	if err := c.get(ctx, path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetDocument returns a single document by ID.
func (c *Client) GetDocument(ctx context.Context, id string) (*Document, error) {
	var doc Document
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/documents/%s", id), &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

// DeleteDocument deletes a document by ID.
func (c *Client) DeleteDocument(ctx context.Context, id string) error {
	return c.delete(ctx, fmt.Sprintf("/api/sdk/documents/%s", id))
}

// Search performs a semantic search across documents.
// If StoreID is set, searches within that store; otherwise searches across all stores.
func (c *Client) Search(ctx context.Context, input SearchInput) ([]SearchResult, error) {
	var resp struct {
		Results []SearchResult `json:"results"`
	}
	var path string
	if input.StoreID != "" {
		path = fmt.Sprintf("/api/sdk/stores/%s/search", input.StoreID)
	} else {
		path = "/api/sdk/search"
	}
	if err := c.post(ctx, path, input, &resp); err != nil {
		return nil, err
	}
	return resp.Results, nil
}
