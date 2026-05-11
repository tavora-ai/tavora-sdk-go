// Package tavora — secret vaults (Stage 3 of the composable-primitives
// plan in tavora-go). Envelope-encrypted, app-scoped vaults of named
// secrets the agent reads via `secret(name)` in the sandbox when its
// session is pinned to a vault.
//
// The SDK never returns plaintext. Set takes a value, encrypts it
// server-side, and returns the redacted view (name + kek_id +
// timestamps). List returns the same redacted shape. There is no
// "get plaintext" endpoint by design — the only way to retrieve a
// secret value is from inside a running agent session that pinned
// the vault.
//
// Endpoints return 503 when the server has no `TAVORA_SECRET_KEK`
// configured (secret vaults disabled).
package tavora

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// SecretVault is one named vault inside an app.
type SecretVault struct {
	ID        string          `json:"id"`
	AppID     string          `json:"app_id"`
	Name      string          `json:"name"`
	Metadata  json.RawMessage `json:"metadata"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// RedactedSecret is the public view of a secret. Never contains the
// plaintext value or the encrypted blob — only metadata safe to put on
// the wire. CreatedAt / UpdatedAt are strings here because the server
// formats them as RFC3339 directly on the redacted struct (rather than
// round-tripping through pgtype.Timestamp). Callers parse if needed.
type RedactedSecret struct {
	VaultID   string `json:"vault_id"`
	Name      string `json:"name"`
	KEKID     string `json:"kek_id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// CreateSecretVaultInput holds the parameters for creating a vault.
type CreateSecretVaultInput struct {
	Name     string          `json:"name"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// UpdateSecretVaultInput updates vault metadata. PATCH semantics:
// omitted Metadata preserves the current value.
type UpdateSecretVaultInput struct {
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// CreateSecretVault creates a named vault.
func (c *Client) CreateSecretVault(ctx context.Context, input CreateSecretVaultInput) (*SecretVault, error) {
	var vault SecretVault
	if err := c.post(ctx, "/api/sdk/secret-vaults", input, &vault); err != nil {
		return nil, err
	}
	return &vault, nil
}

// ListSecretVaults returns every vault in the app.
func (c *Client) ListSecretVaults(ctx context.Context) ([]SecretVault, error) {
	var resp struct {
		SecretVaults []SecretVault `json:"secret_vaults"`
	}
	if err := c.get(ctx, "/api/sdk/secret-vaults", &resp); err != nil {
		return nil, err
	}
	return resp.SecretVaults, nil
}

// GetSecretVault returns one vault by id. Returns vault metadata only —
// not the secrets inside it (use ListSecrets for that).
func (c *Client) GetSecretVault(ctx context.Context, id string) (*SecretVault, error) {
	var vault SecretVault
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/secret-vaults/%s", id), &vault); err != nil {
		return nil, err
	}
	return &vault, nil
}

// UpdateSecretVault patches a vault's metadata.
func (c *Client) UpdateSecretVault(ctx context.Context, id string, input UpdateSecretVaultInput) (*SecretVault, error) {
	var vault SecretVault
	if err := c.patch(ctx, fmt.Sprintf("/api/sdk/secret-vaults/%s", id), input, &vault); err != nil {
		return nil, err
	}
	return &vault, nil
}

// DeleteSecretVault deletes the vault and (by FK cascade) every secret
// inside it. Irreversible — the ciphertext rows are gone.
func (c *Client) DeleteSecretVault(ctx context.Context, id string) error {
	return c.delete(ctx, fmt.Sprintf("/api/sdk/secret-vaults/%s", id))
}

// ListSecrets returns the REDACTED view of every secret in a vault.
// Never returns plaintext.
func (c *Client) ListSecrets(ctx context.Context, vaultID string) ([]RedactedSecret, error) {
	var resp struct {
		Secrets []RedactedSecret `json:"secrets"`
	}
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/secret-vaults/%s/secrets", vaultID), &resp); err != nil {
		return nil, err
	}
	return resp.Secrets, nil
}

// PutSecret encrypts and stores `value` under `name` in the vault.
// Generates a fresh DEK + nonce on every write — required because
// AES-GCM is broken by nonce reuse. Returns the redacted view; the
// plaintext leaves the SDK process only when an agent calls
// `secret(name)` in the sandbox.
func (c *Client) PutSecret(ctx context.Context, vaultID, name, value string) (*RedactedSecret, error) {
	var red RedactedSecret
	path := fmt.Sprintf("/api/sdk/secret-vaults/%s/secrets/%s", vaultID, url.PathEscape(name))
	if err := c.put(ctx, path, map[string]string{"value": value}, &red); err != nil {
		return nil, err
	}
	return &red, nil
}

// DeleteSecret removes one secret. Idempotent — 204 even when absent.
func (c *Client) DeleteSecret(ctx context.Context, vaultID, name string) error {
	return c.delete(ctx, fmt.Sprintf("/api/sdk/secret-vaults/%s/secrets/%s", vaultID, url.PathEscape(name)))
}
