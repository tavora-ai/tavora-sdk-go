package tavora

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// Deployment is a deployment target (dev / staging / prod) for a
// PROJECT — the Convex release model scopes environments to the whole
// project, not a single agent. Slugs are globally unique (Convex
// pattern), so a slug alone resolves a deployment via
// GetDeploymentBySlug. OwnerUserID is set only for dev targets.
type Deployment struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	Kind        string    `json:"kind"` // "dev" | "staging" | "prod"
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	OwnerUserID string    `json:"owner_user_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ListDeployments returns every deployment target for a project,
// prod-first. project is the project slug from tavora.jsonc (the same
// handle used by deploy/promote/status); the server resolves it,
// tenant-scoped.
//
// Endpoint: GET /api/sdk/projects/{project}/deployments
func (c *Client) ListDeployments(ctx context.Context, project string) ([]Deployment, error) {
	var out struct {
		Deployments []Deployment `json:"deployments"`
	}
	path := fmt.Sprintf("/api/sdk/projects/%s/deployments", url.PathEscape(project))
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return out.Deployments, nil
}

// EnsureDevDeployment idempotently returns the caller's personal dev
// deployment for a project, creating it if absent.
//
// Endpoint: POST /api/sdk/projects/{project}/deployments/ensure-dev
func (c *Client) EnsureDevDeployment(ctx context.Context, project string) (*Deployment, error) {
	var out Deployment
	path := fmt.Sprintf("/api/sdk/projects/%s/deployments/ensure-dev", url.PathEscape(project))
	if err := c.post(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetDeploymentBySlug resolves a deployment by its globally-unique slug.
//
// Endpoint: GET /api/sdk/deployments/{slug}
func (c *Client) GetDeploymentBySlug(ctx context.Context, slug string) (*Deployment, error) {
	var out Deployment
	path := fmt.Sprintf("/api/sdk/deployments/%s", url.PathEscape(slug))
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeploymentEnvEntry is the redacted view of one entry on a
// deployment's KV store: key + is_secret + timestamps. Values never
// round-trip on a list — reveal one via GetDeploymentEnv.
type DeploymentEnvEntry struct {
	Key       string    `json:"key"`
	IsSecret  bool      `json:"is_secret"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SetDeploymentEnvInput is the body for PutDeploymentEnv. The server
// envelope-encrypts on receipt; the wire carries plaintext exactly
// once, on this PUT. IsSecret toggles UI redaction + trace-redactor
// inclusion — encryption at rest is on regardless.
type SetDeploymentEnvInput struct {
	Value    string `json:"value"`
	IsSecret bool   `json:"is_secret"`
}

// DeploymentEnvValue is the response of GetDeploymentEnv — one entry's
// plaintext value. Treat it like a PUT body and don't log it.
type DeploymentEnvValue struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	IsSecret bool   `json:"is_secret"`
}

// ListDeploymentEnv returns every env/secret entry on a deployment in
// the redacted view (no values). Use for `tavora env list`. project is
// the project slug; slug is the deployment slug.
//
// Endpoint: GET /api/sdk/projects/{project}/deployments/{slug}/env
func (c *Client) ListDeploymentEnv(ctx context.Context, project, slug string) ([]DeploymentEnvEntry, error) {
	var out struct {
		Env []DeploymentEnvEntry `json:"env"`
	}
	path := fmt.Sprintf("/api/sdk/projects/%s/deployments/%s/env",
		url.PathEscape(project), url.PathEscape(slug))
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return out.Env, nil
}

// GetDeploymentEnv returns one entry with its plaintext value. Use for
// `tavora env get` / `tavora secret get`.
//
// Endpoint: GET /api/sdk/projects/{project}/deployments/{slug}/env/{key}
func (c *Client) GetDeploymentEnv(ctx context.Context, project, slug, key string) (*DeploymentEnvValue, error) {
	var out DeploymentEnvValue
	path := fmt.Sprintf("/api/sdk/projects/%s/deployments/%s/env/%s",
		url.PathEscape(project), url.PathEscape(slug), url.PathEscape(key))
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PutDeploymentEnv upserts one entry. Re-puts rotate the DEK + nonce on
// the server, so AES-GCM stays safe.
//
// Endpoint: PUT /api/sdk/projects/{project}/deployments/{slug}/env/{key}
func (c *Client) PutDeploymentEnv(ctx context.Context, project, slug, key string, input SetDeploymentEnvInput) (*DeploymentEnvEntry, error) {
	var out DeploymentEnvEntry
	path := fmt.Sprintf("/api/sdk/projects/%s/deployments/%s/env/%s",
		url.PathEscape(project), url.PathEscape(slug), url.PathEscape(key))
	if err := c.put(ctx, path, input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteDeploymentEnv removes one entry. Idempotent — a missing key
// succeeds the same as a real delete.
//
// Endpoint: DELETE /api/sdk/projects/{project}/deployments/{slug}/env/{key}
func (c *Client) DeleteDeploymentEnv(ctx context.Context, project, slug, key string) error {
	path := fmt.Sprintf("/api/sdk/projects/%s/deployments/%s/env/%s",
		url.PathEscape(project), url.PathEscape(slug), url.PathEscape(key))
	return c.delete(ctx, path)
}
