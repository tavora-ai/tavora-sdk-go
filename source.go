package tavora

import (
	"context"
	"net/url"
	"time"
)

// SourceFile is one entry inside a SourceSyncManifest. The content
// is base64-encoded by the JSON encoder if the bytes aren't UTF-8
// (rare for source code — agent.jsonc, persona.md, skills/*.js are
// all text). Hash is "sha256:<hex>" so the server can do
// content-addressed dedupe later without changing the contract.
type SourceFile struct {
	Path    string `json:"path"`
	Hash    string `json:"hash"`
	Size    int    `json:"size"`
	Content []byte `json:"content,omitempty"`
}

// SourceAgent is the per-agent slice of the manifest. SourceHash is
// the hash of every (path, content) pair under this agent's folder,
// computed in sorted-path order so it's stable across operating
// systems and CI shards.
type SourceAgent struct {
	ID         string       `json:"id"`
	SourceHash string       `json:"sourceHash"`
	Files      []SourceFile `json:"files"`
}

// SourceSyncManifest is the payload `tavora dev` (or any other
// SourceSync caller) sends to the server on every debounced change.
// The CLI builds it; the server persists a dev draft from it.
type SourceSyncManifest struct {
	Project     string        `json:"project"`
	SourceHash  string        `json:"sourceHash"`
	Agents      []SourceAgent `json:"agents"`
	GeneratedAt time.Time     `json:"generatedAt"`
}

// SourceSyncResult is what the server returns after persisting the
// dev draft. DraftHash matches the manifest's SourceHash on a
// successful round-trip. ServerIssues carry the AI-friendly warnings
// from server-side validation (see SourceValidationIssue); a fatal
// issue surfaces as a 422 APIError instead.
type SourceSyncResult struct {
	DraftHash    string                  `json:"draftHash"`
	Agents       []SourceSyncAgentResult `json:"agents"`
	SyncedAt     time.Time               `json:"syncedAt"`
	ServerIssues []SourceValidationIssue `json:"serverIssues,omitempty"`
}

// SourceSyncAgentResult is per-agent result info — most usefully,
// the local→server agent_id mapping the server maintains, so the
// CLI can show "support → 7f2a…".
type SourceSyncAgentResult struct {
	LocalID    string `json:"localId"`
	AgentID    string `json:"agentId"`
	DraftID    string `json:"draftId"`
	SourceHash string `json:"sourceHash"`
}

// SourceValidationIssue mirrors the AI-friendly Issue type the CLI
// produces locally. The server-side validator returns these for any
// problem that requires its authority (missing project, no agents,
// duplicate ids). Severity is "fatal" or "warning".
type SourceValidationIssue struct {
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Hint     string `json:"hint,omitempty"`
	Severity string `json:"severity"`
}

// SourceSync upserts a dev draft from the supplied manifest.
//
// Endpoint: PUT /api/sdk/source-sync
//
// The server validates the manifest, upserts one agents row per local
// id, and inserts a draft row carrying the manifest. The CLI uses the
// returned hash to confirm the round-trip and the per-agent
// local→server id mapping in SourceSyncResult.Agents.
func (c *Client) SourceSync(ctx context.Context, manifest SourceSyncManifest) (*SourceSyncResult, error) {
	var out SourceSyncResult
	if err := c.put(ctx, "/api/sdk/source-sync", manifest, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SourceDeployInput is the body of SourceDeploy. A deploy is
// project-atomic — it cuts one release spanning every agent in the
// project, so there is no per-agent selector.
type SourceDeployInput struct {
	Project string `json:"project"`
}

// SourceDeployResult is what the server returns after cutting an
// immutable project RELEASE — a snapshot of every agent's latest draft.
// The caller's personal dev deployment is repointed at the new release.
type SourceDeployResult struct {
	ReleaseID     string             `json:"releaseId"`
	ReleaseNumber int64              `json:"releaseNumber"`
	Agents        []ReleaseAgentInfo `json:"agents"`
	DeployedAt    time.Time          `json:"deployedAt"`
}

// ReleaseAgentInfo is one agent captured in a release — the draft
// snapshot the release pinned for that agent.
type ReleaseAgentInfo struct {
	LocalID    string `json:"localId"`
	AgentID    string `json:"agentId"`
	DraftID    string `json:"draftId"`
	SourceHash string `json:"sourceHash"`
}

// SourceDeploy cuts an immutable project release from every agent's
// latest synced draft, and points the caller's dev deployment at it.
// The typical inner loop is `tavora dev` (sync drafts) → `tavora deploy`
// (cut a release). Release numbers auto-increment per project from 1.
//
// Endpoint: POST /api/sdk/source-deploy
func (c *Client) SourceDeploy(ctx context.Context, input SourceDeployInput) (*SourceDeployResult, error) {
	var out SourceDeployResult
	if err := c.post(ctx, "/api/sdk/source-deploy", input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SourcePromoteInput is the body of SourcePromote. To is the target
// environment, "staging" or "prod". ReleaseNumber is optional — omit to
// promote the latest release, or set it to pin (or roll back to) a
// specific release. Promotion is project-atomic (the whole agent set
// moves together), so there is no per-agent selector.
type SourcePromoteInput struct {
	Project       string `json:"project"`
	To            string `json:"to"`
	ReleaseNumber *int64 `json:"releaseNumber,omitempty"`
}

// SourcePromoteResult is what the server returns after repointing an
// environment at a project release.
type SourcePromoteResult struct {
	To             string    `json:"to"`
	DeploymentSlug string    `json:"deploymentSlug"`
	ReleaseID      string    `json:"releaseId"`
	ReleaseNumber  int64     `json:"releaseNumber"`
	PromotedAt     time.Time `json:"promotedAt"`
}

// SourcePromote moves an environment's (staging or prod) release pointer
// for the whole project — the Convex-style "promote a release" step,
// atomic across every agent. Staging is auto-created on first promote;
// prod is auto-resolved.
//
// Endpoint: POST /api/sdk/source-promote
func (c *Client) SourcePromote(ctx context.Context, input SourcePromoteInput) (*SourcePromoteResult, error) {
	var out SourcePromoteResult
	if err := c.post(ctx, "/api/sdk/source-promote", input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SourceStatus reports the project's latest release and the release each
// of its dev/staging/prod environments currently serves — the data
// behind `tavora status`. Promote state is project-wide in the release
// model, so the version numbers live on the top level, not per agent.
type SourceStatus struct {
	Project string            `json:"project"`
	Latest  int64             `json:"latest"`
	Dev     *int64            `json:"dev,omitempty"`
	Staging *int64            `json:"staging,omitempty"`
	Prod    *int64            `json:"prod,omitempty"`
	Agents  []StatusAgentInfo `json:"agents"`
}

// StatusAgentInfo is one agent in the project, for the status display.
type StatusAgentInfo struct {
	LocalID string `json:"localId"`
	AgentID string `json:"agentId"`
}

// SourceStatus fetches the project-level deployment status.
//
// Endpoint: GET /api/sdk/source-status?project=<name>
func (c *Client) SourceStatus(ctx context.Context, project string) (*SourceStatus, error) {
	var out SourceStatus
	if err := c.get(ctx, "/api/sdk/source-status?project="+url.QueryEscape(project), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
