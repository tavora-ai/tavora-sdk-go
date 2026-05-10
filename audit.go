package tavora

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// TenantAuditEntry is one row from the tenant audit log.
type TenantAuditEntry struct {
	ID             string          `json:"id"`
	ProductID    string          `json:"product_id"`
	ActorUserID    *string         `json:"actor_user_id"`
	ActorAPIKeyID  *string         `json:"actor_api_key_id"`
	Action         string          `json:"action"`
	SubjectType    string          `json:"subject_type"`
	SubjectID      string          `json:"subject_id"`
	AgentVersionID *string         `json:"agent_version_id"`
	SessionID      *string         `json:"session_id"`
	Metadata       json.RawMessage `json:"metadata"`
	CreatedAt      time.Time       `json:"created_at"`
}

// AuditListFilter narrows a ListAuditLog call. All fields optional.
type AuditListFilter struct {
	Action      string
	ActorUserID string
	SubjectType string
	From        time.Time // zero = no lower bound
	To          time.Time // zero = no upper bound
	Limit       int       // 0 = server default
	Offset      int
}

// AuditListPage is the paginated list response.
type AuditListPage struct {
	Entries []TenantAuditEntry `json:"entries"`
	Total   int64              `json:"total"`
	Limit   int                `json:"limit"`
	Offset  int                `json:"offset"`
}

// ListAuditLog returns a page of audit entries for the API-key's product.
func (c *Client) ListAuditLog(ctx context.Context, f AuditListFilter) (*AuditListPage, error) {
	q := url.Values{}
	if f.Action != "" {
		q.Set("action", f.Action)
	}
	if f.ActorUserID != "" {
		q.Set("actor_user_id", f.ActorUserID)
	}
	if f.SubjectType != "" {
		q.Set("subject_type", f.SubjectType)
	}
	if !f.From.IsZero() {
		q.Set("from", f.From.UTC().Format(time.RFC3339))
	}
	if !f.To.IsZero() {
		q.Set("to", f.To.UTC().Format(time.RFC3339))
	}
	if f.Limit > 0 {
		q.Set("limit", fmt.Sprint(f.Limit))
	}
	if f.Offset > 0 {
		q.Set("offset", fmt.Sprint(f.Offset))
	}

	path := "/api/sdk/audit-log"
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	var out AuditListPage
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AuditExportFilter narrows the rows returned by ExportAuditLog. Action +
// SubjectType mirror the tenant-audit-log viewer so a CI pipeline can export
// the same slice an operator sees in the UI.
type AuditExportFilter struct {
	Format      string    // "json" (default) or "csv"
	Action      string    // optional exact match on the action name
	SubjectType string    // optional exact match on subject_type
	From        time.Time // zero = no lower bound
	To          time.Time // zero = no upper bound
}

// ExportAuditLog returns the raw export bytes (CSV or JSON) matching the
// given filter. Callers typically write the bytes to disk for SOC2
// evidence collection.
func (c *Client) ExportAuditLog(ctx context.Context, f AuditExportFilter) ([]byte, error) {
	format := f.Format
	if format == "" {
		format = "json"
	}
	q := url.Values{}
	q.Set("format", format)
	if f.Action != "" {
		q.Set("action", f.Action)
	}
	if f.SubjectType != "" {
		q.Set("subject_type", f.SubjectType)
	}
	if !f.From.IsZero() {
		q.Set("from", f.From.UTC().Format(time.RFC3339))
	}
	if !f.To.IsZero() {
		q.Set("to", f.To.UTC().Format(time.RFC3339))
	}
	resp, err := c.resty.R().SetContext(ctx).Get("/api/sdk/audit-log/export?" + q.Encode())
	if err != nil {
		return nil, fmt.Errorf("tavora: export request failed: %w", err)
	}
	if err := checkError(resp); err != nil {
		return nil, err
	}
	return resp.Body(), nil
}
