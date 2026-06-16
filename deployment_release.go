package tavora

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// deployment_release.go covers the Tier 3 project-environment release
// model: a deployment (dev / staging / prod) pins one published version
// per agent. Cutting a release diffs each agent by source_hash — an
// unchanged agent keeps its current version, a changed one gets a new
// version — and pins the result into the environment. Promotion copies
// one environment's pin set into another (typically staging → prod).
//
// These hit the project-scoped routes the platform serves on BOTH the
// protected (JWT) and SDK (X-API-Key) APIs; the SDK client uses the
// /api/sdk/* surface.

// ProjectEnvironment mirrors the platform deployments DTO — a project
// environment holding a pin set + a shared env/secret store.
type ProjectEnvironment struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	Project     string    `json:"project"`
	Kind        string    `json:"kind"` // "dev" | "staging" | "prod"
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	OwnerUserID string    `json:"owner_user_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// VersionPin is one (environment, agent) → published-version binding.
type VersionPin struct {
	DeploymentID  string    `json:"deployment_id"`
	AgentID       string    `json:"agent_id"`
	VersionID     string    `json:"version_id"`
	VersionNumber int64     `json:"version_number"`
	PromotedBy    string    `json:"promoted_by,omitempty"`
	PromotedAt    time.Time `json:"promoted_at"`
}

// Release is the result of a cut/promote/rollback: the target environment
// plus its full pin set afterwards, and the history row the operation
// recorded (ReleaseID/Seq/Action — see ReleaseRecord).
type Release struct {
	Deployment ProjectEnvironment `json:"deployment"`
	Pins       []VersionPin       `json:"pins"`
	ReleaseID  string             `json:"release_id,omitempty"`
	Seq        int64              `json:"seq,omitempty"`
	Action     string             `json:"action,omitempty"` // "cut" | "promote" | "rollback"
}

// ReleaseRecord is one entry in an environment's append-only release
// history: a named {agent -> version} snapshot a rollback can re-apply.
type ReleaseRecord struct {
	ID              string    `json:"id"`
	DeploymentID    string    `json:"deployment_id"`
	Seq             int64     `json:"seq"`
	Action          string    `json:"action"` // "cut" | "promote" | "rollback"
	SourceReleaseID string    `json:"source_release_id,omitempty"`
	Note            string    `json:"note,omitempty"`
	CreatedBy       string    `json:"created_by,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	AgentCount      int64     `json:"agent_count"`
}

// CutRelease cuts a release into the project's STAGING environment,
// creating it if absent. Unchanged agents keep their version; changed
// agents get a new one. Production is reached only via PromoteDeployment
// — staging is the sole place new versions are cut, so prod always runs
// exactly what was validated in staging.
//
// Endpoint: POST /api/sdk/projects/{project}/deployments
func (c *Client) CutRelease(ctx context.Context, project string) (*Release, error) {
	var out Release
	path := fmt.Sprintf("/api/sdk/projects/%s/deployments", url.PathEscape(project))
	if err := c.post(ctx, path, map[string]string{}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PromoteDeployment copies an environment's pin set into a target
// environment (default prod), creating it if absent. No new versions are
// cut — it's a pure re-pin. slug identifies the source environment.
//
// Endpoint: POST /api/sdk/projects/{project}/deployments/{slug}/promote
func (c *Client) PromoteDeployment(ctx context.Context, project, slug, toKind string) (*Release, error) {
	var out Release
	body := map[string]string{}
	if toKind != "" {
		body["to_kind"] = toKind
	}
	path := fmt.Sprintf("/api/sdk/projects/%s/deployments/%s/promote",
		url.PathEscape(project), url.PathEscape(slug))
	if err := c.post(ctx, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListDeploymentPins returns an environment's agent → version pin set.
//
// Endpoint: GET /api/sdk/projects/{project}/deployments/{slug}/pins
func (c *Client) ListDeploymentPins(ctx context.Context, project, slug string) ([]VersionPin, error) {
	var out struct {
		Pins []VersionPin `json:"pins"`
	}
	path := fmt.Sprintf("/api/sdk/projects/%s/deployments/%s/pins",
		url.PathEscape(project), url.PathEscape(slug))
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return out.Pins, nil
}

// ListReleases returns an environment's release history, newest first —
// the snapshots Rollback can re-apply.
//
// Endpoint: GET /api/sdk/projects/{project}/deployments/{slug}/releases
func (c *Client) ListReleases(ctx context.Context, project, slug string) ([]ReleaseRecord, error) {
	var out struct {
		Releases []ReleaseRecord `json:"releases"`
	}
	path := fmt.Sprintf("/api/sdk/projects/%s/deployments/%s/releases",
		url.PathEscape(project), url.PathEscape(slug))
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return out.Releases, nil
}

// Rollback re-applies a prior release's pin set into its environment,
// recording a fresh rollback release. releaseID must belong to the
// (project, slug) environment. No new versions are cut — a pure re-pin.
//
// Endpoint: POST /api/sdk/projects/{project}/deployments/{slug}/rollback
func (c *Client) Rollback(ctx context.Context, project, slug, releaseID string) (*Release, error) {
	var out Release
	path := fmt.Sprintf("/api/sdk/projects/%s/deployments/%s/rollback",
		url.PathEscape(project), url.PathEscape(slug))
	if err := c.post(ctx, path, map[string]string{"release_id": releaseID}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
