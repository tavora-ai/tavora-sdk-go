package tavora

import (
	"context"
	"net/http"
	"testing"
)

func TestGetMetrics(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/metrics", 200, ProductMetrics{
		Tokens: TokenMetrics{
			PromptTokens:    1000,
			CandidateTokens: 500,
			TotalTokens:     1500,
			RequestCount:    10,
		},
		Agents: AgentMetrics{
			TotalSessions:     50,
			ActiveSessions:    3,
			CompletedSessions: 45,
			ErrorSessions:     2,
			TotalSteps:        200,
		},
		Evals: EvalMetrics{
			TotalRuns:     5,
			CompletedRuns: 4,
			TotalPassed:   18,
			TotalFailed:   2,
			AverageScore:  8.2,
		},
	})

	m, err := ts.client().GetMetrics(context.Background())
	assertNoError(t, err)

	assertEqual(t, "total_tokens", m.Tokens.TotalTokens, int64(1500))
	assertEqual(t, "request_count", m.Tokens.RequestCount, int64(10))
	assertEqual(t, "total_sessions", m.Agents.TotalSessions, int64(50))
	assertEqual(t, "active_sessions", m.Agents.ActiveSessions, int64(3))
	assertEqual(t, "eval_runs", m.Evals.TotalRuns, int64(5))
	assertEqual(t, "avg_score", m.Evals.AverageScore, 8.2)

	req := ts.lastRequest(t)
	assertEqual(t, "path", req.Path, "/api/sdk/metrics")
}
