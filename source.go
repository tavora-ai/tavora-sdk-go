package tavora

import (
	"context"
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
//
// Environment is the per-developer env id (server-managed; the CLI
// leaves it blank to mean "the API key's default dev environment").
type SourceSyncManifest struct {
	Project     string        `json:"project"`
	Environment string        `json:"environment,omitempty"`
	SourceHash  string        `json:"sourceHash"`
	Agents      []SourceAgent `json:"agents"`
	GeneratedAt time.Time     `json:"generatedAt"`
}

// SourceSyncResult is what the server returns after persisting the
// dev draft. DraftHash matches the manifest's SourceHash on a
// successful round-trip. ServerErrors carry the AI-friendly error
// payloads from server-side validation (see
// SourceValidationIssue). On a 200 response ServerErrors is nil.
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
// problem that requires its authority (missing index, model
// unavailable, secretRef not declared, tier limits). See the v0
// concept doc, §Validation.
type SourceValidationIssue struct {
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Hint     string `json:"hint,omitempty"`
	Severity string `json:"severity"`
}

// SourceDeployResult is what the server returns after promoting a
// dev draft to a published version. One entry per agent the deploy
// covered.
type SourceDeployResult struct {
	Version      string              `json:"version"`
	Agents       []DeployedAgentInfo `json:"agents"`
	DeployedAt   time.Time           `json:"deployedAt"`
	ServerIssues []SourceValidationIssue `json:"serverIssues,omitempty"`
}

type DeployedAgentInfo struct {
	LocalID   string `json:"localId"`
	AgentID   string `json:"agentId"`
	VersionID string `json:"versionId"`
	Semver    string `json:"semver"`
}

// SourceExport is the payload `tavora pull` consumes — the inverse
// of SourceSyncManifest. Reuses the same file shape so a round-trip
// is symmetric.
type SourceExport struct {
	Project string        `json:"project"`
	Agents  []SourceAgent `json:"agents"`
}

// SourceDiff is what `tavora diff` returns — paths that differ and
// the side they differ on. Empty Paths means "in sync".
type SourceDiff struct {
	InSync bool             `json:"inSync"`
	Paths  []SourceDiffPath `json:"paths"`
}

type SourceDiffPath struct {
	Path  string `json:"path"`
	State string `json:"state"` // "local-only" | "server-only" | "changed"
}

// SourceSync upserts a dev draft from the supplied manifest.
//
// Endpoint: PUT /api/sdk/source-sync
//
// The server validates the manifest, persists a single
// (agent, environment, kind='draft') row per agent in
// agent_versions, and returns the draft hash. The CLI uses the hash
// to confirm the round-trip and to drive the per-agent local→server
// id mapping in SourceSyncResult.Agents.
func (c *Client) SourceSync(ctx context.Context, manifest SourceSyncManifest) (*SourceSyncResult, error) {
	var out SourceSyncResult
	if err := c.put(ctx, "/api/sdk/source-sync", manifest, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SourceValidate runs the server-side validator against a manifest
// without persisting anything. Useful for CI dry-runs and for the
// `tavora deploy --dry-run` flag.
//
// Endpoint: POST /api/sdk/source-validate
func (c *Client) SourceValidate(ctx context.Context, manifest SourceSyncManifest) ([]SourceValidationIssue, error) {
	var out struct {
		Issues []SourceValidationIssue `json:"issues"`
	}
	if err := c.post(ctx, "/api/sdk/source-validate", manifest, &out); err != nil {
		return nil, err
	}
	return out.Issues, nil
}

// SourceDeploy promotes the most recent dev draft for the given
// project to an immutable published version. The server reads the
// latest draft per agent, validates, and appends a kind='published'
// row. Atomically updates the production routing pointer.
//
// Endpoint: POST /api/sdk/source-deploy
//
// Project must match the manifest the draft was synced with.
// AgentID is optional — pass to deploy a single agent (the
// "per-agent escape hatch" called out in the concept doc).
func (c *Client) SourceDeploy(ctx context.Context, input SourceDeployInput) (*SourceDeployResult, error) {
	var out SourceDeployResult
	if err := c.post(ctx, "/api/sdk/source-deploy", input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SourceDeployInput is the body of SourceDeploy.
type SourceDeployInput struct {
	Project     string `json:"project"`
	Environment string `json:"environment,omitempty"`
	// LocalAgentID limits the deploy to a single agent. Empty
	// deploys all agents in the project.
	LocalAgentID string `json:"localAgentId,omitempty"`
	// RunEvals overrides the agent.jsonc deploy.runEvals setting.
	// Tri-state: nil = leave to agent.jsonc, &true = force run,
	// &false = skip even if agent.jsonc requested it.
	RunEvals *bool `json:"runEvals,omitempty"`
}

// SourceExport returns the latest server-side state for the given
// project as a SourceExport. The CLI's `tavora pull` writes the
// returned files to disk.
//
// Endpoint: GET /api/sdk/source-export?project=<name>
func (c *Client) SourceExport(ctx context.Context, project string) (*SourceExport, error) {
	var out SourceExport
	if err := c.get(ctx, "/api/sdk/source-export?project="+project, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SourceRenameInput is the body of SourceRename. The verb tells the
// server "I changed `agent.jsonc:id` from old to new — preserve the
// binding"; without it the next sync looks like a delete + create.
type SourceRenameInput struct {
	Project    string `json:"project"`
	OldLocalID string `json:"oldLocalId"`
	NewLocalID string `json:"newLocalId"`
}

type SourceRenameResult struct {
	AgentID    string `json:"agentId"`
	OldLocalID string `json:"oldLocalId"`
	NewLocalID string `json:"newLocalId"`
}

// SourceRename updates the code-first local_id of an existing agent.
//
// Endpoint: POST /api/sdk/source-rename
//
// 409 if NewLocalID already exists in the project; 404 if
// OldLocalID isn't found.
func (c *Client) SourceRename(ctx context.Context, input SourceRenameInput) (*SourceRenameResult, error) {
	var out SourceRenameResult
	if err := c.post(ctx, "/api/sdk/source-rename", input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SourceDeleteInput is the body of SourceDelete. Force must be true;
// the server returns "force_required" otherwise. Cascades to
// agent_versions, sessions, eval runs.
type SourceDeleteInput struct {
	Project string `json:"project"`
	LocalID string `json:"localId"`
	Force   bool   `json:"force"`
}

type SourceDeleteResult struct {
	AgentID string `json:"agentId"`
	LocalID string `json:"localId"`
	Deleted bool   `json:"deleted"`
}

// SourceDelete destroys an agent that was source-managed and all
// the rows that depend on it. Irreversible.
//
// Endpoint: POST /api/sdk/source-delete
func (c *Client) SourceDelete(ctx context.Context, input SourceDeleteInput) (*SourceDeleteResult, error) {
	var out SourceDeleteResult
	if err := c.post(ctx, "/api/sdk/source-delete", input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SourceDiff returns the differences between the supplied manifest
// and the latest published version on the server. Drives `tavora
// diff`.
//
// Endpoint: POST /api/sdk/source-diff
func (c *Client) SourceDiff(ctx context.Context, manifest SourceSyncManifest) (*SourceDiff, error) {
	var out SourceDiff
	if err := c.post(ctx, "/api/sdk/source-diff", manifest, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
