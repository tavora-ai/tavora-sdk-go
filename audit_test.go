package tavora

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestListAuditLog(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/audit-log", 200, AuditListPage{
		Entries: []TenantAuditEntry{
			{ID: "a_1", Action: "promotion.propose"},
			{ID: "a_2", Action: "promotion.approve"},
		},
		Total: 2, Limit: 100, Offset: 0,
	})

	page, err := ts.client().ListAuditLog(context.Background(), AuditListFilter{})
	assertNoError(t, err)
	assertEqual(t, "count", len(page.Entries), 2)
	assertEqual(t, "total", int(page.Total), 2)
}

func TestListAuditLog_AppliesFilters(t *testing.T) {
	ts := newTestServer(t)
	// Register on the base path with the default handler; the query string is
	// asserted after the call.
	ts.on(http.MethodGet, "/api/sdk/audit-log", 200, AuditListPage{})

	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	_, err := ts.client().ListAuditLog(context.Background(), AuditListFilter{
		Action: "promotion.approve",
		From:   from, To: to,
		Limit: 25, Offset: 50,
	})
	assertNoError(t, err)

	req := ts.lastRequest(t)
	if !strings.Contains(req.Path, "action=promotion.approve") {
		t.Fatalf("expected action filter in path, got %q", req.Path)
	}
	if !strings.Contains(req.Path, "from=2026-01-01T00%3A00%3A00Z") {
		t.Fatalf("expected from= in path, got %q", req.Path)
	}
	if !strings.Contains(req.Path, "limit=25") || !strings.Contains(req.Path, "offset=50") {
		t.Fatalf("expected limit/offset in path, got %q", req.Path)
	}
}

func TestExportAuditLog(t *testing.T) {
	ts := newTestServer(t)
	ts.onRaw(http.MethodGet, "/api/sdk/audit-log/export", 200,
		`[{"id":"a_1","action":"promotion.approve"}]`)

	body, err := ts.client().ExportAuditLog(context.Background(), AuditExportFilter{})
	assertNoError(t, err)
	var decoded []TenantAuditEntry
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal export: %v", err)
	}
	assertEqual(t, "count", len(decoded), 1)
	assertEqual(t, "id", decoded[0].ID, "a_1")

	req := ts.lastRequest(t)
	if !strings.Contains(req.Path, "format=json") {
		t.Fatalf("expected format=json in path, got %q", req.Path)
	}
}

func TestExportAuditLog_AppliesFilters(t *testing.T) {
	ts := newTestServer(t)
	ts.onRaw(http.MethodGet, "/api/sdk/audit-log/export", 200, `[]`)

	_, err := ts.client().ExportAuditLog(context.Background(), AuditExportFilter{
		Format:      "csv",
		Action:      "promotion.approve",
		SubjectType: "promotion",
	})
	assertNoError(t, err)

	req := ts.lastRequest(t)
	for _, want := range []string{"format=csv", "action=promotion.approve", "subject_type=promotion"} {
		if !strings.Contains(req.Path, want) {
			t.Fatalf("expected %q in path, got %q", want, req.Path)
		}
	}
}
