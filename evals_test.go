package tavora

import (
	"context"
	"net/http"
	"testing"
)

// CreateEvalCase / DeleteEvalCase / RunEval tests were removed
// 2026-05-17 along with the methods themselves. Eval cases are
// authored in tavora/agents/<id>/evals/*.json and arrive through
// source-sync; coverage for that upsert path lives in
// tavora-go's TestSourceSync_UpsertsEvalCases.

func TestListEvalCases(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/evals", 200, map[string]interface{}{
		"cases": []EvalCase{
			{ID: "ec_1", Name: "test-1"},
		},
	})

	cases, err := ts.client().ListEvalCases(context.Background())
	assertNoError(t, err)
	assertEqual(t, "count", len(cases), 1)
}

func TestListEvalRuns(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/eval-runs", 200, map[string]interface{}{
		"runs": []EvalRun{
			{ID: "er_1", Status: "completed", Passed: 4, Failed: 1},
		},
	})

	runs, err := ts.client().ListEvalRuns(context.Background())
	assertNoError(t, err)
	assertEqual(t, "count", len(runs), 1)
	assertEqual(t, "passed", runs[0].Passed, int32(4))
}

func TestGetEvalRun(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/eval-runs/er_1", 200, EvalRunDetail{
		Run: EvalRun{ID: "er_1", Status: "completed", AverageScore: 8.5},
		Results: []EvalResult{
			{ID: "res_1", CaseName: "test-1", Score: 9, Pass: true},
			{ID: "res_2", CaseName: "test-2", Score: 8, Pass: true},
		},
	})

	detail, err := ts.client().GetEvalRun(context.Background(), "er_1")
	assertNoError(t, err)
	assertEqual(t, "run id", detail.Run.ID, "er_1")
	assertEqual(t, "avg score", detail.Run.AverageScore, float32(8.5))
	assertEqual(t, "results", len(detail.Results), 2)
	assertEqual(t, "first pass", detail.Results[0].Pass, true)
}
