package tavora

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// AgentConfig is a persistent agent configuration owned by a project.
// The live config (persona, skills, stores, provider, model) lives
// on the agent row directly; AgentVersion rows are append-only
// history snapshots written by the code-first publish path
// (`/api/sdk/source-deploy`).
//
// Named AgentConfig in the SDK to distinguish from AgentSession, which
// is an ephemeral run. The backend uses the URL segment "agent-configs"
// for the same reason.
type AgentConfig struct {
	ID          string `json:"id"`
	ProjectID       string `json:"project_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedBy   string `json:"created_by"`

	// Live config — the runtime reads these for new sessions.
	PersonaMD           string          `json:"persona_md"`
	SkillsJSON          json.RawMessage `json:"skills_json"`
	StoresJSON          json.RawMessage `json:"stores_json"`
	Provider            string          `json:"provider"`
	Model               string          `json:"model"`
	EnabledCapabilities []string        `json:"enabled_capabilities"`

	// Per-agent operator setting. The previous RunEvalOnPublish toggle
	// is gone — the browser no longer publishes, and run-on-deploy
	// lives on the CLI as `tavora deploy --run-evals`.
	EvalSuiteID *string `json:"eval_suite_id"`

	ActiveVersionID *string    `json:"active_version_id"`
	PublishedAt     *time.Time `json:"published_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`

	// Code-first markers, non-nil when the agent is managed by
	// `tavora dev` from a local tavora/ folder. CodeFirstProject is
	// the project name from tavora.jsonc; CodeFirstLocalID is the
	// `id` field in agent.jsonc. SDK callers use these to look up
	// an agent UUID by local id without having to source-sync first
	// (`ListAgentConfigs` + filter).
	CodeFirstProject *string `json:"code_first_project,omitempty"`
	CodeFirstLocalID *string `json:"code_first_local_id,omitempty"`
}

// SkillBinding pins a skill at a specific version inside an AgentVersion.
type SkillBinding struct {
	SkillID string `json:"skill_id"`
	Version string `json:"version"`
}

// AgentVersion is an immutable snapshot of an AgentConfig. Append-only
// history; one row per publish.
type AgentVersion struct {
	ID               string          `json:"id"`
	AgentID          string          `json:"agent_id"`
	Semver           string          `json:"semver"`
	PersonaMD        string          `json:"persona_md"`
	SkillsJSON       json.RawMessage `json:"skills_json"`
	StoresJSON       json.RawMessage `json:"stores_json"`
	Provider         string          `json:"provider"`
	Model            string          `json:"model"`
	EvalSuiteID      *string         `json:"eval_suite_id"`
	EvalSuiteVersion *string         `json:"eval_suite_version"`
	CreatedBy        string          `json:"created_by"`
	CreatedAt        time.Time       `json:"created_at"`
}

// DraftConfig was removed 2026-05-17 along with the UpdateAgentDraft /
// DiscardAgentDraft / PublishAgent / RevertAgent SDK methods. Drafts
// are an SDK-internal concept owned by the code-first source-sync
// path; the agent's deployed config is the only shape SDK callers
// inspect.

// EvalRunResult wraps the row created by RunAgentEval. Wrapped in a
// struct so future fields (e.g. estimated_duration_s) can land without
// breaking callers.
type EvalRunResult struct {
	Run EvalRun `json:"run"`
}

// --- Input types ---
//
// CreateAgentConfigInput was removed on 2026-05-16 with the pivot
// to a Convex-style code-first authoring model: agents land in
// the database only via SourceSync.

// UpdateAgentSettingsInput patches per-agent operator settings.
// EvalSuiteID="" clears the pin; nil leaves it alone. The previous
// RunEvalOnPublish toggle is gone — see AgentConfig for the
// rationale.
type UpdateAgentSettingsInput struct {
	EvalSuiteID *string `json:"eval_suite_id,omitempty"`
}

// --- AgentConfig methods ---
//
// CreateAgentConfig was removed on 2026-05-16 — see the note on
// CreateAgentConfigInput above. Use Client.SourceSync (source.go)
// to create agents through the code-first path.

func (c *Client) ListAgentConfigs(ctx context.Context) ([]AgentConfig, error) {
	var out []AgentConfig
	if err := c.get(ctx, "/api/sdk/agent-configs", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetAgentConfig(ctx context.Context, agentID string) (*AgentConfig, error) {
	var out AgentConfig
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/agent-configs/%s", agentID), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteAgentConfig(ctx context.Context, agentID string) error {
	return c.delete(ctx, fmt.Sprintf("/api/sdk/agent-configs/%s", agentID))
}

// --- AgentVersion methods ---
//
// UpdateAgentConfig (rename/describe via REST), SetActiveAgentVersion,
// and CreateAgentVersion (direct version creation) were removed when
// code-first took over. To cut a version use SourceDeploy; to ship one
// to an environment use SourcePromote (Convex-style promote).

func (c *Client) ListAgentVersions(ctx context.Context, agentID string) ([]AgentVersion, error) {
	var out []AgentVersion
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/agent-configs/%s/versions", agentID), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetAgentVersion(ctx context.Context, agentID, versionID string) (*AgentVersion, error) {
	var out AgentVersion
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/agent-configs/%s/versions/%s", agentID, versionID), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- Draft + publish ---
//
// UpdateAgentDraft / DiscardAgentDraft / PublishAgent / RevertAgent
// were removed 2026-05-17. The draft + publish endpoints they called
// were retired in the UI rethink; authoring lives in the local
// `tavora/` folder and arrives via SourceSync / SourceDeploy.

// --- Settings + advisory eval ---

// UpdateAgentSettings patches per-agent operator settings. Pass
// EvalSuiteID=&"" to clear the pin; nil leaves a field unchanged.
func (c *Client) UpdateAgentSettings(ctx context.Context, agentID string, input UpdateAgentSettingsInput) (*AgentConfig, error) {
	var out AgentConfig
	if err := c.patch(ctx, fmt.Sprintf("/api/sdk/agent-configs/%s/settings", agentID), input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RunAgentEval triggers an advisory async eval against the agent's
// pinned suite using the deployed (live) persona. The previous
// target=draft variant was removed with the UI rethink; CLI users
// that want to evaluate the synced dev draft run
// `tavora evals run --draft`.
func (c *Client) RunAgentEval(ctx context.Context, agentID string) (*EvalRunResult, error) {
	path := fmt.Sprintf("/api/sdk/agent-configs/%s/eval-runs", agentID)
	var out EvalRunResult
	if err := c.post(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListAgentEvalRuns returns the most-recent N eval runs for the
// agent's pinned suite. Pass 0 for the server default (5); max 50.
func (c *Client) ListAgentEvalRuns(ctx context.Context, agentID string, limit int) ([]EvalRun, error) {
	path := fmt.Sprintf("/api/sdk/agent-configs/%s/eval-runs", agentID)
	if limit > 0 {
		path += fmt.Sprintf("?limit=%d", limit)
	}
	var out []EvalRun
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return out, nil
}
