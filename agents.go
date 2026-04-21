package tavora

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// AgentSession represents a server-side agent session.
type AgentSession struct {
	ID               string          `json:"id"`
	WorkspaceID      string          `json:"workspace_id"`
	Title            string          `json:"title"`
	SystemPrompt     string          `json:"system_prompt"`
	Model            string          `json:"model"`
	ToolsConfig      json.RawMessage `json:"tools_config"`
	Metadata         json.RawMessage `json:"metadata"`
	Status           string          `json:"status"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	PromptTokens     int64           `json:"prompt_tokens"`
	CompletionTokens int64           `json:"completion_tokens"`
	StepCount        int32           `json:"step_count"`
	DurationMs       int32           `json:"duration_ms"`
	FinishedAt       *time.Time      `json:"finished_at,omitempty"`
}

// AgentStep represents a step in the agent's reasoning.
type AgentStep struct {
	ID               string          `json:"id"`
	SessionID        string          `json:"session_id"`
	StepIndex        int32           `json:"step_index"`
	StepType         string          `json:"step_type"`
	Content          string          `json:"content"`
	ToolName         string          `json:"tool_name"`
	ToolArgs         json.RawMessage `json:"tool_args"`
	DurationMs       int32           `json:"duration_ms"`
	CreatedAt        time.Time       `json:"created_at"`
	PromptTokens     int32           `json:"prompt_tokens"`
	CompletionTokens int32           `json:"completion_tokens"`
}

// AgentSessionDetail includes session and its steps.
type AgentSessionDetail struct {
	Session AgentSession `json:"session"`
	Steps   []AgentStep  `json:"steps"`
}

// CreateAgentSessionInput holds parameters for creating an agent session.
type CreateAgentSessionInput struct {
	Title        string          `json:"title,omitempty"`
	SystemPrompt string          `json:"system_prompt,omitempty"`
	Model        string          `json:"model,omitempty"`
	Tools        []string        `json:"tools,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
}

// AgentEvent represents a step event during agent execution.
type AgentEvent struct {
	Type    string         `json:"type"`
	Content string         `json:"content,omitempty"`
	Tool    string         `json:"tool,omitempty"`
	Args    map[string]any `json:"args,omitempty"`
	Result  any            `json:"result,omitempty"`
	Summary *RunSummary    `json:"summary,omitempty"`
}

// RunSummary holds aggregate metrics for an agent run.
type RunSummary struct {
	SessionID string `json:"session_id"`
	Steps     int    `json:"steps"`
	Tokens    struct {
		Prompt     int32 `json:"prompt"`
		Completion int32 `json:"completion"`
	} `json:"tokens"`
}

// GetAgentSystemPrompt returns the full system prompt the agent sees for this space.
func (c *Client) GetAgentSystemPrompt(ctx context.Context) (string, error) {
	var resp struct {
		Prompt string `json:"prompt"`
	}
	if err := c.get(ctx, "/api/sdk/agents/system-prompt", &resp); err != nil {
		return "", err
	}
	return resp.Prompt, nil
}

// CreateAgentSession creates a new agent session.
func (c *Client) CreateAgentSession(ctx context.Context, input CreateAgentSessionInput) (*AgentSession, error) {
	var session AgentSession
	if err := c.post(ctx, "/api/sdk/agents", input, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// ListAgentSessions returns agent sessions in the space.
func (c *Client) ListAgentSessions(ctx context.Context, limit, offset int) ([]AgentSession, error) {
	if limit <= 0 {
		limit = 50
	}
	var resp struct {
		Sessions []AgentSession `json:"sessions"`
	}
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/agents?limit=%d&offset=%d", limit, offset), &resp); err != nil {
		return nil, err
	}
	return resp.Sessions, nil
}

// GetAgentSession returns an agent session with its steps.
func (c *Client) GetAgentSession(ctx context.Context, id string) (*AgentSessionDetail, error) {
	var detail AgentSessionDetail
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/agents/%s", id), &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

// DeleteAgentSession deletes an agent session.
func (c *Client) DeleteAgentSession(ctx context.Context, id string) error {
	return c.delete(ctx, fmt.Sprintf("/api/sdk/agents/%s", id))
}

// RunAgent sends a message to an agent session and streams execution events.
func (c *Client) RunAgent(ctx context.Context, sessionID, message string, onEvent func(AgentEvent)) error {
	resp, err := c.resty.R().
		SetContext(ctx).
		SetHeader("Accept", "text/event-stream").
		SetBody(map[string]string{"message": message}).
		SetDoNotParseResponse(true).
		Post(fmt.Sprintf("/api/sdk/agents/%s/run", sessionID))
	if err != nil {
		return fmt.Errorf("tavora: agent run request failed: %w", err)
	}
	defer resp.RawBody().Close()

	if resp.StatusCode() >= 400 {
		body, _ := io.ReadAll(resp.RawBody())
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err != nil {
			return &APIError{StatusCode: resp.StatusCode(), Message: resp.Status()}
		}
		apiErr.StatusCode = resp.StatusCode()
		return &apiErr
	}

	return parseSSEStream(resp.RawBody(), onEvent)
}

func parseSSEStream(reader io.Reader, onEvent func(AgentEvent)) error {
	scanner := bufio.NewScanner(reader)
	var currentEvent string

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			var evt AgentEvent
			if err := json.Unmarshal([]byte(data), &evt); err != nil {
				continue
			}

			onEvent(evt)

			if currentEvent == "done" || currentEvent == "error" {
				return nil
			}
			currentEvent = ""
		}
	}

	return scanner.Err()
}
