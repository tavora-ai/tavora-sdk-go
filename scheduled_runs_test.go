package tavora

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestListScheduledRuns(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/scheduled-runs", 200, map[string]interface{}{
		"scheduled_runs": []ScheduledRun{
			{ID: "sr_1", Name: "Nightly report", CronExpression: "0 0 * * *"},
		},
	})

	runs, err := ts.client().ListScheduledRuns(context.Background())
	assertNoError(t, err)
	assertEqual(t, "count", len(runs), 1)
	assertEqual(t, "name", runs[0].Name, "Nightly report")
}

func TestGetScheduledRun(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/scheduled-runs/sr_1", 200, ScheduledRun{
		ID:             "sr_1",
		CronExpression: "0 0 * * *",
		Enabled:        true,
		RunCount:       42,
	})

	run, err := ts.client().GetScheduledRun(context.Background(), "sr_1")
	assertNoError(t, err)
	assertEqual(t, "id", run.ID, "sr_1")
	assertEqual(t, "run_count", run.RunCount, int32(42))
	assertEqual(t, "enabled", run.Enabled, true)
}

func TestCreateScheduledRun(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/scheduled-runs", 201, ScheduledRun{
		ID:             "sr_new",
		AgentSessionID: "as_1",
		CronExpression: "*/5 * * * *",
		Message:        "Check status",
	})

	run, err := ts.client().CreateScheduledRun(context.Background(), CreateScheduledRunInput{
		AgentSessionID: "as_1",
		Name:           "Status check",
		CronExpression: "*/5 * * * *",
		Message:        "Check status",
	})
	assertNoError(t, err)
	assertEqual(t, "id", run.ID, "sr_new")

	req := ts.lastRequest(t)
	var body CreateScheduledRunInput
	json.Unmarshal([]byte(req.Body), &body)
	assertEqual(t, "agent_session_id", body.AgentSessionID, "as_1")
	assertEqual(t, "cron", body.CronExpression, "*/5 * * * *")
}

func TestDeleteScheduledRun(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodDelete, "/api/sdk/scheduled-runs/sr_1", 204, nil)

	err := ts.client().DeleteScheduledRun(context.Background(), "sr_1")
	assertNoError(t, err)
}
