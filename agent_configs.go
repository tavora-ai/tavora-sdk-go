package tavora

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// AgentConfig is a persistent agent configuration owned by an app.
// Post-agent-simplification the live config (persona, skills, stores,
// provider, model) lives on the agent row directly — what `AgentVersion`
// used to be the only source of. AgentVersion rows are now append-only
// history snapshots written on each publish.
//
// Named AgentConfig in the SDK to distinguish from AgentSession, which
// is an ephemeral run. The backend uses the URL segment "agent-configs"
// for the same reason.
type AgentConfig struct {
	ID          string `json:"id"`
	AppID       string `json:"app_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedBy   string `json:"created_by"`

	// Live config — the runtime reads these for new sessions. Mirrored
	// onto agent_versions on each publish.
	PersonaMD           string          `json:"persona_md"`
	SkillsJSON          json.RawMessage `json:"skills_json"`
	StoresJSON          json.RawMessage `json:"stores_json"`
	Provider            string          `json:"provider"`
	Model               string          `json:"model"`
	EnabledCapabilities []string        `json:"enabled_capabilities"`

	// Per-agent operator settings (PR5).
	EvalSuiteID      *string `json:"eval_suite_id"`
	RunEvalOnPublish bool    `json:"run_eval_on_publish"`

	// Draft slot — non-nil when the operator has staged unpublished
	// edits. The runtime is unaffected by the draft.
	DraftConfig *DraftConfig `json:"draft_config"`

	ActiveVersionID *string    `json:"active_version_id"`
	PublishedAt     *time.Time `json:"published_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
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

// DraftConfig is the staged next-state of an agent. The frontend always
// sends the complete intended state — partial-merge isn't supported, so
// callers should construct this fully (typically from an existing live
// config).
type DraftConfig struct {
	PersonaMD           string         `json:"persona_md"`
	Skills              []SkillBinding `json:"skills"`
	Stores              []string       `json:"stores"`
	Provider            string         `json:"provider"`
	Model               string         `json:"model"`
	EnabledCapabilities []string       `json:"enabled_capabilities,omitempty"`
	EvalSuiteID         string         `json:"eval_suite_id,omitempty"`
	EvalSuiteVersion    string         `json:"eval_suite_version,omitempty"`
}

// PublishResult is what Publish and Revert return — the updated agent
// row plus the new history snapshot that was just appended.
type PublishResult struct {
	Agent   AgentConfig  `json:"agent"`
	Version AgentVersion `json:"version"`
}

// EvalRunResult wraps the row created by RunAgentEval. Wrapped in a
// struct so future fields (e.g. estimated_duration_s) can land without
// breaking callers.
type EvalRunResult struct {
	Run EvalRun `json:"run"`
}

// --- Input types ---

type CreateAgentConfigInput struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type UpdateAgentConfigInput struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// CreateAgentVersionInput creates a new version directly. Less common
// in the post-simplification world (publish does this automatically);
// kept for callers that build history snapshots out of band.
type CreateAgentVersionInput struct {
	FromVersionID    string         `json:"from_version_id,omitempty"`
	Semver           string         `json:"semver,omitempty"`
	PersonaMD        string         `json:"persona_md,omitempty"`
	Skills           []SkillBinding `json:"skills,omitempty"`
	Stores           []string       `json:"stores,omitempty"`
	Provider         string         `json:"provider,omitempty"`
	Model            string         `json:"model,omitempty"`
	EvalSuiteID      string         `json:"eval_suite_id,omitempty"`
	EvalSuiteVersion string         `json:"eval_suite_version,omitempty"`
}

// UpdateAgentSettingsInput patches per-agent operator settings.
// EvalSuiteID="" clears the pin; nil leaves it alone. Same for
// RunEvalOnPublish (nil = leave alone).
type UpdateAgentSettingsInput struct {
	EvalSuiteID      *string `json:"eval_suite_id,omitempty"`
	RunEvalOnPublish *bool   `json:"run_eval_on_publish,omitempty"`
}

// EvalTarget selects which persona an advisory eval uses for its
// sessions. "live" reads the published persona; "draft" reads the
// staged draft and 409s when nothing is staged.
type EvalTarget string

const (
	EvalTargetLive  EvalTarget = "live"
	EvalTargetDraft EvalTarget = "draft"
)

// --- AgentConfig methods ---

func (c *Client) CreateAgentConfig(ctx context.Context, input CreateAgentConfigInput) (*AgentConfig, error) {
	var out AgentConfig
	if err := c.post(ctx, "/api/sdk/agent-configs", input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

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

func (c *Client) UpdateAgentConfig(ctx context.Context, agentID string, input UpdateAgentConfigInput) (*AgentConfig, error) {
	var out AgentConfig
	if err := c.patch(ctx, fmt.Sprintf("/api/sdk/agent-configs/%s", agentID), input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteAgentConfig(ctx context.Context, agentID string) error {
	return c.delete(ctx, fmt.Sprintf("/api/sdk/agent-configs/%s", agentID))
}

// SetActiveAgentVersion pins an active version on the AgentConfig. In
// practice the publish/revert path is what callers want; this is the
// low-level door for callers that need direct control.
func (c *Client) SetActiveAgentVersion(ctx context.Context, agentID, versionID string) (*AgentConfig, error) {
	body := map[string]string{"version_id": versionID}
	var out AgentConfig
	if err := c.put(ctx, fmt.Sprintf("/api/sdk/agent-configs/%s/active-version", agentID), body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- AgentVersion methods ---

func (c *Client) CreateAgentVersion(ctx context.Context, agentID string, input CreateAgentVersionInput) (*AgentVersion, error) {
	var out AgentVersion
	if err := c.post(ctx, fmt.Sprintf("/api/sdk/agent-configs/%s/versions", agentID), input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

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

// --- Draft + publish (PR3 of agent simplification) ---

// UpdateAgentDraft stages a complete proposed next-state in the
// agent's draft_config. The runtime is unaffected; the live config
// keeps serving sessions until Publish.
func (c *Client) UpdateAgentDraft(ctx context.Context, agentID string, draft DraftConfig) (*AgentConfig, error) {
	var out AgentConfig
	if err := c.patch(ctx, fmt.Sprintf("/api/sdk/agent-configs/%s/draft", agentID), draft, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DiscardAgentDraft clears the staged draft. Idempotent — discarding
// when no draft exists is a no-op (still writes the audit row).
func (c *Client) DiscardAgentDraft(ctx context.Context, agentID string) (*AgentConfig, error) {
	var out AgentConfig
	if err := c.deleteWithResult(ctx, fmt.Sprintf("/api/sdk/agent-configs/%s/draft", agentID), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PublishAgent promotes the staged draft to live in one transaction:
// appends an immutable agent_versions snapshot, mirrors the new live
// columns, clears draft_config, audits. Returns the updated agent +
// the freshly-appended history row. 409 when no draft exists.
func (c *Client) PublishAgent(ctx context.Context, agentID string) (*PublishResult, error) {
	var out PublishResult
	if err := c.post(ctx, fmt.Sprintf("/api/sdk/agent-configs/%s/publish", agentID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RevertAgent publishes a named historical version as the new live
// config. Same audit/version-append semantics as Publish — a revert
// is a publish whose source is an existing history row.
func (c *Client) RevertAgent(ctx context.Context, agentID, versionID string) (*PublishResult, error) {
	body := map[string]string{"version_id": versionID}
	var out PublishResult
	if err := c.post(ctx, fmt.Sprintf("/api/sdk/agent-configs/%s/revert", agentID), body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- Settings + advisory eval (PR5) ---

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
// pinned suite. target=EvalTargetDraft uses the staged persona instead
// of the published one so callers can compare scores before publishing.
// Pass an empty target for the default (live).
func (c *Client) RunAgentEval(ctx context.Context, agentID string, target EvalTarget) (*EvalRunResult, error) {
	path := fmt.Sprintf("/api/sdk/agent-configs/%s/eval-runs", agentID)
	if target != "" {
		path += "?target=" + url.QueryEscape(string(target))
	}
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
