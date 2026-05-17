package tavora

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// EvalCase represents an eval test case.
type EvalCase struct {
	ID            string          `json:"id"`
	AppID         string          `json:"app_id"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	SetName       string          `json:"set_name"`
	Type          string          `json:"type"`
	Config        json.RawMessage `json:"config"`
	Prompt        string          `json:"prompt"`
	Criteria      string          `json:"criteria"`
	SystemPrompt  string          `json:"system_prompt"`
	Tools         json.RawMessage `json:"tools"`
	PassThreshold int32           `json:"pass_threshold"`
	CreatedAt     time.Time       `json:"created_at"`
}

// EvalRun represents an eval suite execution.
type EvalRun struct {
	ID           string    `json:"id"`
	Status       string    `json:"status"`
	TotalCases   int32     `json:"total_cases"`
	Passed       int32     `json:"passed"`
	Failed       int32     `json:"failed"`
	AverageScore float32   `json:"average_score"`
	JudgeModel   string    `json:"judge_model"`
	CreatedAt    time.Time `json:"created_at"`
}

// EvalResult represents a single case result within a run.
type EvalResult struct {
	ID         string          `json:"id"`
	CaseName   string          `json:"case_name"`
	SetName    string          `json:"set_name"`
	Score      int32           `json:"score"`
	Pass       bool            `json:"pass"`
	Reasoning  string          `json:"reasoning"`
	Response   string          `json:"response"`
	ToolCalls  json.RawMessage `json:"tool_calls"`
	DurationMs int32           `json:"duration_ms"`
	Error      string          `json:"error"`
}

// EvalRunDetail includes run + results.
type EvalRunDetail struct {
	Run     EvalRun      `json:"run"`
	Results []EvalResult `json:"results"`
}

// CreateEvalCase / DeleteEvalCase / RunEval (the cross-suite trigger)
// were removed 2026-05-17. Eval cases are authored in
// tavora/agents/<id>/evals/*.json and arrive via `tavora dev`. To
// run evals against an agent's pinned suite, use the per-agent
// trigger on `/api/sdk/agent-configs/:id/eval-runs` (call
// `RunAgentConfigEval` on the agent-configs client).

func (c *Client) ListEvalCases(ctx context.Context) ([]EvalCase, error) {
	var resp struct {
		Cases []EvalCase `json:"cases"`
	}
	if err := c.get(ctx, "/api/sdk/evals", &resp); err != nil {
		return nil, err
	}
	return resp.Cases, nil
}

func (c *Client) ListEvalRuns(ctx context.Context) ([]EvalRun, error) {
	var resp struct {
		Runs []EvalRun `json:"runs"`
	}
	if err := c.get(ctx, "/api/sdk/eval-runs", &resp); err != nil {
		return nil, err
	}
	return resp.Runs, nil
}

func (c *Client) GetEvalRun(ctx context.Context, id string) (*EvalRunDetail, error) {
	var detail EvalRunDetail
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/eval-runs/%s", id), &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}
