package tavora

import (
	"context"
	"fmt"
	"net/url"
)

// DeploymentEnvEntry is the redacted view of one entry on a
// deployment's KV store: key + is_secret + timestamps. Values
// never round-trip through the wire on a list — write-only by
// design. Use the values themselves only when you have the
// plaintext to PUT.
type DeploymentEnvEntry struct {
	DeploymentID string `json:"deployment_id"`
	Key          string `json:"key"`
	IsSecret     bool   `json:"is_secret"`
	KEKID        string `json:"kek_id"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// SetDeploymentEnvInput is the body for PutDeploymentEnv. The
// server encrypts on receipt; the wire carries plaintext exactly
// once, on this PUT.
//
// IsSecret toggles UI redaction and trace-redactor inclusion at
// runtime. Encryption at rest is on regardless — flipping the flag
// later is a column update with no re-key.
type SetDeploymentEnvInput struct {
	Value    string `json:"value"`
	IsSecret bool   `json:"is_secret"`
}

// DeploymentEnvValue is the response shape of GetDeploymentEnv —
// one entry's plaintext value. Distinct from the redacted
// DeploymentEnvEntry returned by List, which deliberately omits
// the value. The wire carries plaintext on this response; treat
// the return like the request body of a PUT and don't log it.
type DeploymentEnvValue struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	IsSecret bool   `json:"is_secret"`
}

// ListDeploymentEnv returns every entry on the named deployment in
// the redacted view. Use this for `tavora env list`.
//
// Endpoint: GET /api/sdk/deployments/{slug}/env
func (c *Client) ListDeploymentEnv(ctx context.Context, slug string) ([]DeploymentEnvEntry, error) {
	var out struct {
		Env []DeploymentEnvEntry `json:"env"`
	}
	path := fmt.Sprintf("/api/sdk/deployments/%s/env", url.PathEscape(slug))
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return out.Env, nil
}

// GetDeploymentEnv returns one entry with its plaintext value.
// Use this for `tavora env get` / `tavora secret get` and for
// shell substitution patterns like
//
//	export X=$(tavora env get FOO)
//
// Endpoint: GET /api/sdk/deployments/{slug}/env/{key}
func (c *Client) GetDeploymentEnv(ctx context.Context, slug, key string) (*DeploymentEnvValue, error) {
	var out DeploymentEnvValue
	path := fmt.Sprintf("/api/sdk/deployments/%s/env/%s",
		url.PathEscape(slug), url.PathEscape(key))
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PutDeploymentEnv upserts one entry. Re-puts of the same key
// rotate the DEK + nonce on the server, so AES-GCM stays safe.
//
// Endpoint: PUT /api/sdk/deployments/{slug}/env/{key}
func (c *Client) PutDeploymentEnv(ctx context.Context, slug, key string, input SetDeploymentEnvInput) (*DeploymentEnvEntry, error) {
	var out DeploymentEnvEntry
	path := fmt.Sprintf("/api/sdk/deployments/%s/env/%s",
		url.PathEscape(slug), url.PathEscape(key))
	if err := c.put(ctx, path, input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteDeploymentEnv removes one entry. Idempotent — missing key
// returns 204 the same as a successful delete.
//
// Endpoint: DELETE /api/sdk/deployments/{slug}/env/{key}
func (c *Client) DeleteDeploymentEnv(ctx context.Context, slug, key string) error {
	path := fmt.Sprintf("/api/sdk/deployments/%s/env/%s",
		url.PathEscape(slug), url.PathEscape(key))
	return c.delete(ctx, path)
}
