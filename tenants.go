// Package tavora — tenant facade (Stage 5 of the composable-primitives
// plan in tavora-go). Customers pass an opaque `tenant_ref` string on
// session create and the platform isolates state (memory, secrets,
// audit, future rate limits) behind it.
//
// The Tenants resource is for pre-provisioning + admin. Most callers
// never use it directly — first-touch session-create auto-provisions
// the tenant. The endpoints exist for backfills from a customer's own
// user table and for admin / debugging.
//
// Opaque means opaque: the platform never models the customer's
// user/org schema. Pass any UTF-8 string, 1–256 bytes. The platform
// records what primitives it auto-created behind it and uses that
// mapping for stable resolution across sessions.
package tavora

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// Tenant is the resolved view of a tenant_pins row.
type Tenant struct {
	TenantRef     string    `json:"tenant_ref"`
	IndexIDs      []string  `json:"index_ids,omitempty"`
	MemoryStoreID *string   `json:"memory_store_id,omitempty"`
	SecretVaultID *string   `json:"secret_vault_id,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
}

// ProvisionTenantInput is the explicit-create input. Most callers
// don't use this — session-create auto-provisions. Use it for backfills
// or when you need to opt out of auto memory_store / secret_vault.
type ProvisionTenantInput struct {
	TenantRef     string          `json:"tenant_ref"`
	NoMemoryStore bool            `json:"no_memory_store,omitempty"`
	NoSecretVault bool            `json:"no_secret_vault,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
}

// UpdateTenantInput overrides pinned refs / metadata on an existing
// active tenant. Pointer fields distinguish "omit (preserve)" from
// "set to null (clear)". Pass `IndexIDs: &[]string{}` to clear the
// pin (deny-all retrieval); leave nil to preserve.
type UpdateTenantInput struct {
	IndexIDs      *[]string       `json:"index_ids,omitempty"`
	MemoryStoreID *string         `json:"memory_store_id,omitempty"`
	SecretVaultID *string         `json:"secret_vault_id,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
}

// ProvisionTenant explicitly creates a tenant_pins row. Idempotent —
// returns the existing row if already provisioned.
func (c *Client) ProvisionTenant(ctx context.Context, input ProvisionTenantInput) (*Tenant, error) {
	var t Tenant
	if err := c.post(ctx, "/api/sdk/tenants", input, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// ListTenants returns every active tenant in the app.
func (c *Client) ListTenants(ctx context.Context) ([]Tenant, error) {
	var resp struct {
		Tenants []Tenant `json:"tenants"`
	}
	if err := c.get(ctx, "/api/sdk/tenants", &resp); err != nil {
		return nil, err
	}
	return resp.Tenants, nil
}

// GetTenant resolves (app, tenant_ref). Lazy-creates the tenant if
// first touch — same semantics as session-create's resolve path.
func (c *Client) GetTenant(ctx context.Context, tenantRef string) (*Tenant, error) {
	var t Tenant
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/tenants/%s", url.PathEscape(tenantRef)), &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// UpdateTenant overrides pinned refs / metadata.
func (c *Client) UpdateTenant(ctx context.Context, tenantRef string, input UpdateTenantInput) (*Tenant, error) {
	var t Tenant
	path := fmt.Sprintf("/api/sdk/tenants/%s", url.PathEscape(tenantRef))
	if err := c.patch(ctx, path, input, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// ArchiveTenant soft-deletes the pin. The canonical tenant_ref slot is
// freed — a later session-create with the same tenant_ref will
// lazy-create a fresh tenant. Existing sessions and audit rows retain
// the original tenant_ref via independent copies, so the audit trail
// isn't lost.
func (c *Client) ArchiveTenant(ctx context.Context, tenantRef string) error {
	return c.delete(ctx, fmt.Sprintf("/api/sdk/tenants/%s", url.PathEscape(tenantRef)))
}
