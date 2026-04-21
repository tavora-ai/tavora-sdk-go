package tavora

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestCreateEvalCase(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/evals", 201, EvalCase{
		ID:       "ec_1",
		Name:     "search-test",
		Prompt:   "Find pricing docs",
		Criteria: "Must use search tool",
	})

	threshold := int32(7)
	ec, err := ts.client().CreateEvalCase(context.Background(), CreateEvalCaseInput{
		Name:          "search-test",
		Prompt:        "Find pricing docs",
		Criteria:      "Must use search tool",
		SetName:       "smoke",
		Tools:         []string{"search"},
		PassThreshold: &threshold,
	})
	assertNoError(t, err)
	assertEqual(t, "id", ec.ID, "ec_1")
	assertEqual(t, "name", ec.Name, "search-test")

	req := ts.lastRequest(t)
	var body CreateEvalCaseInput
	json.Unmarshal([]byte(req.Body), &body)
	assertEqual(t, "set_name", body.SetName, "smoke")
	assertEqual(t, "tools count", len(body.Tools), 1)
}

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

func TestDeleteEvalCase(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodDelete, "/api/sdk/evals/ec_1", 204, nil)

	err := ts.client().DeleteEvalCase(context.Background(), "ec_1")
	assertNoError(t, err)
}

func TestRunEval(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/evals/run", 200, EvalRun{
		ID:         "er_1",
		Status:     "running",
		TotalCases: 5,
	})

	run, err := ts.client().RunEval(context.Background(), RunEvalInput{
		SetFilter: "smoke",
	})
	assertNoError(t, err)
	assertEqual(t, "id", run.ID, "er_1")
	assertEqual(t, "status", run.Status, "running")
	assertEqual(t, "total_cases", run.TotalCases, int32(5))

	req := ts.lastRequest(t)
	var body RunEvalInput
	json.Unmarshal([]byte(req.Body), &body)
	assertEqual(t, "set_filter", body.SetFilter, "smoke")
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
