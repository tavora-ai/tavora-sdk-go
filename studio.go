package tavora

import (
	"context"
	"encoding/json"
	"fmt"
)

// StudioTrace is the enriched trace returned by the Studio API.
type StudioTrace struct {
	Session      AgentSession      `json:"session"`
	Steps        []json.RawMessage `json:"steps"`
	SystemPrompt string            `json:"system_prompt"`
	Tools        []string          `json:"tools"`
	Memory       map[string]string `json:"memory"`
}

// StudioReplayConfig holds parameters for replaying from a step.
type StudioReplayConfig struct {
	FromStep     int32   `json:"from_step"`
	SystemPrompt *string `json:"system_prompt,omitempty"`
	Message      *string `json:"message,omitempty"`
}

// StudioFixRequest holds parameters for AI fix analysis.
type StudioFixRequest struct {
	FailedSteps     []int  `json:"failed_steps"`
	ExpectedOutcome string `json:"expected_outcome"`
}

// StudioFixSuggestion holds AI-generated fix recommendations.
type StudioFixSuggestion struct {
	PromptChanges string `json:"prompt_changes"`
	EvalCase      *struct {
		Name     string `json:"name"`
		Prompt   string `json:"prompt"`
		Criteria string `json:"criteria"`
	} `json:"eval_case,omitempty"`
	Reasoning string `json:"reasoning"`
}

// GetStudioTrace returns an enriched trace for Studio debugging.
func (c *Client) GetStudioTrace(ctx context.Context, sessionID string) (*StudioTrace, error) {
	var trace StudioTrace
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/studio/%s", sessionID), &trace); err != nil {
		return nil, err
	}
	return &trace, nil
}

// ReplayFromStep replays an agent session from a specific step.
// Returns the new session ID via the SSE stream.
func (c *Client) ReplayFromStep(ctx context.Context, sessionID string, config StudioReplayConfig, onEvent func(AgentEvent)) error {
	resp, err := c.resty.R().
		SetContext(ctx).
		SetHeader("Accept", "text/event-stream").
		SetBody(config).
		SetDoNotParseResponse(true).
		Post(fmt.Sprintf("/api/sdk/studio/%s/replay", sessionID))
	if err != nil {
		return fmt.Errorf("tavora: replay request failed: %w", err)
	}
	defer resp.RawBody().Close()

	return parseSSEStream(resp.RawBody(), onEvent)
}

// AnalyzeFix sends a trace to Gemini for AI fix analysis.
func (c *Client) AnalyzeFix(ctx context.Context, sessionID string, req StudioFixRequest) (*StudioFixSuggestion, error) {
	var suggestion StudioFixSuggestion
	if err := c.post(ctx, fmt.Sprintf("/api/sdk/studio/%s/analyze", sessionID), req, &suggestion); err != nil {
		return nil, err
	}
	return &suggestion, nil
}
