package tavora

import (
	"context"
	"fmt"
	"net/url"
)

// Collection identifies a product-scoped JSON document collection
// and reports its current document count.
type Collection struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// CollectionDocument is the SDK shape for one document in a collection.
// _id is server-assigned by the underlying BIGSERIAL row id and is
// included on every find/findOne result.
type CollectionDocument map[string]any

// FindCollectionInput controls a query. Filter values are either plain
// equality (e.g. {"role": "engineer"}) or operator objects:
//
//	{"age": {"$gte": 30, "$lt": 40}}
//	{"role": {"$in": ["engineer", "designer"]}}
//
// Supported operators: $gt, $gte, $lt, $lte, $ne, $in.
//
// Sort: field name, prefix with "-" for descending. Limit/Skip do
// what they say.
type FindCollectionInput struct {
	Filter map[string]any `json:"filter,omitempty"`
	Sort   string         `json:"sort,omitempty"`
	Limit  int            `json:"limit,omitempty"`
	Skip   int            `json:"skip,omitempty"`
}

// UpdateCollectionInput merges Updates into every document matching
// Filter (mongo-style $set semantics — keys at the top level overwrite,
// nothing nested gets merged). Returns the number of documents touched.
type UpdateCollectionInput struct {
	Filter  map[string]any `json:"filter"`
	Updates map[string]any `json:"updates"`
}

// RemoveCollectionInput deletes every document matching Filter. An empty
// filter is rejected by the server — use DropCollection for that.
type RemoveCollectionInput struct {
	Filter map[string]any `json:"filter"`
}

// ListCollections returns every collection in the product with its
// current document count.
func (c *Client) ListCollections(ctx context.Context) ([]Collection, error) {
	var resp struct {
		Collections []Collection `json:"collections"`
	}
	if err := c.get(ctx, "/api/sdk/collections", &resp); err != nil {
		return nil, err
	}
	return resp.Collections, nil
}

// CreateCollection idempotently reserves a collection name. Returns the
// (possibly pre-existing) collection record. Lets callers reserve a
// bucket before inserting any documents — same lazy-create semantics
// the JS surface (collection(name)) gives the agent.
func (c *Client) CreateCollection(ctx context.Context, name string) (*Collection, error) {
	var resp struct {
		Collection Collection `json:"collection"`
	}
	body := map[string]string{"name": name}
	if err := c.post(ctx, "/api/sdk/collections", body, &resp); err != nil {
		return nil, err
	}
	return &resp.Collection, nil
}

// DropCollection removes a collection and every document in it.
// Idempotent — dropping a missing collection is not an error.
func (c *Client) DropCollection(ctx context.Context, name string) error {
	return c.delete(ctx, fmt.Sprintf("/api/sdk/collections/%s", url.PathEscape(name)))
}

// InsertCollectionDocument stores a single document and returns its
// server-assigned _id.
func (c *Client) InsertCollectionDocument(ctx context.Context, name string, doc CollectionDocument) (int64, error) {
	var resp struct {
		ID int64 `json:"id"`
	}
	body := map[string]any{"document": doc}
	if err := c.post(ctx, fmt.Sprintf("/api/sdk/collections/%s/documents", url.PathEscape(name)), body, &resp); err != nil {
		return 0, err
	}
	return resp.ID, nil
}

// InsertCollectionDocuments stores a batch in order and returns the
// assigned _ids in the same order. Stops at the first server error.
func (c *Client) InsertCollectionDocuments(ctx context.Context, name string, docs []CollectionDocument) ([]int64, error) {
	var resp struct {
		IDs []int64 `json:"ids"`
	}
	body := map[string]any{"documents": docs}
	if err := c.post(ctx, fmt.Sprintf("/api/sdk/collections/%s/documents", url.PathEscape(name)), body, &resp); err != nil {
		return nil, err
	}
	return resp.IDs, nil
}

// FindCollectionDocuments runs a filter+opts query against a collection.
// A nil/empty input returns every document in the bucket.
func (c *Client) FindCollectionDocuments(ctx context.Context, name string, input FindCollectionInput) ([]CollectionDocument, error) {
	var resp struct {
		Documents []CollectionDocument `json:"documents"`
	}
	if err := c.post(ctx, fmt.Sprintf("/api/sdk/collections/%s/find", url.PathEscape(name)), input, &resp); err != nil {
		return nil, err
	}
	return resp.Documents, nil
}

// UpdateCollectionDocuments merges Updates into every document matching
// Filter. Returns the count of documents touched.
func (c *Client) UpdateCollectionDocuments(ctx context.Context, name string, input UpdateCollectionInput) (int, error) {
	var resp struct {
		Updated int `json:"updated"`
	}
	if err := c.post(ctx, fmt.Sprintf("/api/sdk/collections/%s/update", url.PathEscape(name)), input, &resp); err != nil {
		return 0, err
	}
	return resp.Updated, nil
}

// RemoveCollectionDocuments deletes every document matching the input
// filter. Returns the count of documents removed.
func (c *Client) RemoveCollectionDocuments(ctx context.Context, name string, input RemoveCollectionInput) (int, error) {
	var resp struct {
		Removed int `json:"removed"`
	}
	if err := c.post(ctx, fmt.Sprintf("/api/sdk/collections/%s/remove", url.PathEscape(name)), input, &resp); err != nil {
		return 0, err
	}
	return resp.Removed, nil
}
