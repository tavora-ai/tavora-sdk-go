package tavora

import "context"

// ProductMetrics contains aggregated metrics for a space.
type ProductMetrics struct {
	Tokens TokenMetrics `json:"tokens"`
	Agents AgentMetrics `json:"agents"`
	Evals  EvalMetrics  `json:"evals"`
}

// TokenMetrics contains token usage aggregates.
type TokenMetrics struct {
	PromptTokens    int64 `json:"prompt_tokens"`
	CandidateTokens int64 `json:"candidate_tokens"`
	TotalTokens     int64 `json:"total_tokens"`
	RequestCount    int64 `json:"request_count"`
}

// AgentMetrics contains agent session aggregates.
type AgentMetrics struct {
	TotalSessions     int64 `json:"total_sessions"`
	ActiveSessions    int64 `json:"active_sessions"`
	CompletedSessions int64 `json:"completed_sessions"`
	ErrorSessions     int64 `json:"error_sessions"`
	TotalSteps        int64 `json:"total_steps"`
}

// EvalMetrics contains eval run aggregates.
type EvalMetrics struct {
	TotalRuns     int64   `json:"total_runs"`
	CompletedRuns int64   `json:"completed_runs"`
	TotalPassed   int64   `json:"total_passed"`
	TotalFailed   int64   `json:"total_failed"`
	AverageScore  float64 `json:"average_score"`
}

// GetMetrics returns aggregated metrics for the space.
func (c *Client) GetMetrics(ctx context.Context) (*ProductMetrics, error) {
	var m ProductMetrics
	if err := c.get(ctx, "/api/sdk/metrics", &m); err != nil {
		return nil, err
	}
	return &m, nil
}
