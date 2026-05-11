package tavora

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// AgentConfig is a persistent agent configuration owned by an app.
// Named AgentConfig in the SDK to distinguish from AgentSession, which is
// an ephemeral run. The backend uses the URL segment "agent-configs" for
// the same reason.
type AgentConfig struct {
	ID              string    `json:"id"`
	AppID     string    `json:"app_id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	CreatedBy       string    `json:"created_by"`
	ActiveVersionID *string   `json:"active_version_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// SkillBinding pins a skill at a specific version inside an AgentVersion.
type SkillBinding struct {
	SkillID string `json:"skill_id"`
	Version string `json:"version"`
}

// AgentVersion is an immutable snapshot of an AgentConfig.
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

// AgentDeployment pins a version to a target (api | channel_binding | none).
type AgentDeployment struct {
	ID         string    `json:"id"`
	AgentID    string    `json:"agent_id"`
	VersionID  string    `json:"version_id"`
	TargetType string    `json:"target_type"`
	TargetRef  string    `json:"target_ref"`
	Status     string    `json:"status"`
	DeployedBy string    `json:"deployed_by"`
	DeployedAt time.Time `json:"deployed_at"`
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

// CreateAgentVersionInput creates a new version. If FromVersionID is set the
// server performs copy-on-write from that version (non-empty fields here
// override; zero fields inherit). Otherwise a stand-alone version is created
// and Model is required.
type CreateAgentVersionInput struct {
	FromVersionID    string         `json:"from_version_id,omitempty"`
	Semver           string         `json:"semver,omitempty"` // auto-bumps if empty
	PersonaMD        string         `json:"persona_md,omitempty"`
	Skills           []SkillBinding `json:"skills,omitempty"`
	Stores           []string       `json:"stores,omitempty"`
	Provider         string         `json:"provider,omitempty"`
	Model            string         `json:"model,omitempty"`
	EvalSuiteID      string         `json:"eval_suite_id,omitempty"`
	EvalSuiteVersion string         `json:"eval_suite_version,omitempty"`
}

type UpsertDeploymentInput struct {
	VersionID  string `json:"version_id"`
	TargetType string `json:"target_type,omitempty"` // defaults to "api"
	TargetRef  string `json:"target_ref,omitempty"`
}

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

// SetActiveAgentVersion pins an active version on the AgentConfig. Future
// phases will guard this behind eval-gated promotion.
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

// --- AgentDeployment methods ---

func (c *Client) UpsertAgentDeployment(ctx context.Context, agentID string, input UpsertDeploymentInput) (*AgentDeployment, error) {
	var out AgentDeployment
	if err := c.post(ctx, fmt.Sprintf("/api/sdk/agent-configs/%s/deployments", agentID), input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListAgentDeployments(ctx context.Context, agentID string) ([]AgentDeployment, error) {
	var out []AgentDeployment
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/agent-configs/%s/deployments", agentID), &out); err != nil {
		return nil, err
	}
	return out, nil
}
