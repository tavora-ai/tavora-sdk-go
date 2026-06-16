package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/fetchpolicy"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/routes"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/scenarios"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/server"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/store"
)

// newFakeback writes a db.json to a tempdir, opens the store, and
// returns the httptest.Server pointing at a freshly built handler.
func newFakeback(t *testing.T, db string) *httptest.Server {
	t.Helper()
	return newFakebackWithRoutes(t, db, nil)
}

// newFakebackWithRoutes is like newFakeback but also mounts the
// supplied custom routes ahead of the CRUD layer.
func newFakebackWithRoutes(t *testing.T, db string, custom []routes.Route) *httptest.Server {
	return newFakebackFull(t, db, custom, nil)
}

// newFakebackFull is the most-knobbed variant: db.json, custom
// routes, and fetchpolicy expectations.
func newFakebackFull(t *testing.T, db string, custom []routes.Route, expected map[string]string) *httptest.Server {
	return newFakebackEverything(t, db, custom, expected, nil)
}

// newFakebackEverything also wires a scenarios registry in.
func newFakebackEverything(t *testing.T, db string, custom []routes.Route, expected map[string]string, scenReg *scenarios.Registry) *httptest.Server {
	t.Helper()
	dir := t.TempDir()
	if db != "" {
		if err := os.WriteFile(filepath.Join(dir, "db.json"), []byte(db), 0o644); err != nil {
			t.Fatalf("write db.json: %v", err)
		}
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	st, err := store.Open(store.Config{
		File:       filepath.Join(dir, "db.json"),
		IDStrategy: "int",
	}, logger)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	h, err := server.New(server.Options{
		Logger:          logger,
		Store:           st,
		Routes:          custom,
		ExpectedHeaders: expected,
		Scenarios:       scenReg,
	})
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts
}

// scenarioDirWith writes one scenario yaml per name and returns the dir.
func scenarioDirWith(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const sampleDB = `{
  "boards": [
    {"id":"b1","title":"Today","ownerId":"u1"},
    {"id":"b2","title":"Backlog","ownerId":"u1"}
  ],
  "cards": [
    {"id":"c1","listId":"l1","title":"Buy milk"},
    {"id":"c2","listId":"l1","title":"Walk dog"}
  ]
}`

func TestFakeBackendHeaderOnEveryResponse(t *testing.T) {
	ts := newFakeback(t, sampleDB)
	resp, err := http.Get(ts.URL + "/boards")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("X-Tavora-Fake-Backend"); got != "true" {
		t.Errorf("X-Tavora-Fake-Backend = %q, want %q", got, "true")
	}
}

func TestListReturnsAllRecords(t *testing.T) {
	ts := newFakeback(t, sampleDB)
	resp, err := http.Get(ts.URL + "/boards")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	var boards []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&boards); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(boards) != 2 {
		t.Errorf("got %d boards, want 2", len(boards))
	}
}

func TestListFilterByFieldEquality(t *testing.T) {
	ts := newFakeback(t, sampleDB)
	resp, err := http.Get(ts.URL + "/cards?listId=l1")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	var cards []map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&cards)
	if len(cards) != 2 {
		t.Errorf("got %d cards, want 2", len(cards))
	}
	resp2, err := http.Get(ts.URL + "/cards?listId=NEVER")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp2.Body.Close()
	cards = nil
	_ = json.NewDecoder(resp2.Body).Decode(&cards)
	if len(cards) != 0 {
		t.Errorf("got %d cards for nonexistent filter, want 0", len(cards))
	}
}

func TestCreatePostAssignsIDAndLocationHeader(t *testing.T) {
	ts := newFakeback(t, sampleDB)
	resp, err := http.Post(ts.URL+"/cards", "application/json",
		strings.NewReader(`{"listId":"l1","title":"new"}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want 201", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); !strings.HasPrefix(loc, "/cards/") {
		t.Errorf("Location = %q, want /cards/...", loc)
	}
	var card map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&card)
	if card["id"] == nil || card["id"] == "" {
		t.Errorf("created card has no id: %v", card)
	}
}

func TestPatchMergesFields(t *testing.T) {
	ts := newFakeback(t, sampleDB)
	req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/cards/c1",
		strings.NewReader(`{"title":"Buy oat milk"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH: %v", err)
	}
	defer resp.Body.Close()
	var card map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&card)
	if card["title"] != "Buy oat milk" {
		t.Errorf("title = %v, want %q", card["title"], "Buy oat milk")
	}
	// Untouched fields stay.
	if card["listId"] != "l1" {
		t.Errorf("listId = %v, want l1", card["listId"])
	}
}

func TestDeleteRemovesRecord(t *testing.T) {
	ts := newFakeback(t, sampleDB)
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/cards/c1", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
	// 404 on follow-up get.
	resp, err = http.Get(ts.URL + "/cards/c1")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d after delete, want 404", resp.StatusCode)
	}
}

func TestAdminResetRestoresSeedState(t *testing.T) {
	ts := newFakeback(t, sampleDB)
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/cards/c1", nil)
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()

	resp, err := http.Post(ts.URL+"/_admin/reset", "", nil)
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	resp.Body.Close()

	// c1 is back.
	resp, err = http.Get(ts.URL + "/cards/c1")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status after reset = %d, want 200", resp.StatusCode)
	}
}

func TestUnknownResourceReturns404(t *testing.T) {
	ts := newFakeback(t, sampleDB)
	resp, err := http.Get(ts.URL + "/widgets")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestCustomRouteServesCannedBody(t *testing.T) {
	ts := newFakebackWithRoutes(t, sampleDB, []routes.Route{
		{
			Method: "POST", Path: "/search", Status: 200,
			Body: map[string]any{
				"results": []map[string]any{
					{"id": "card_1", "title": "Buy milk"},
					{"id": "card_2", "title": "Walk dog"},
				},
			},
		},
	})
	resp, err := http.Post(ts.URL+"/search", "application/json", strings.NewReader(`{"q":"foo"}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	results, ok := body["results"].([]any)
	if !ok || len(results) != 2 {
		t.Errorf("results malformed: %v", body)
	}
}

func TestCustomRouteWinsOverCRUDOnConflict(t *testing.T) {
	// db.json has a `boards` collection that would normally serve
	// GET /boards. A custom route for the same path should win.
	ts := newFakebackWithRoutes(t, sampleDB, []routes.Route{
		{
			Method: "GET", Path: "/boards", Status: 200,
			Body: map[string]any{"override": true, "boards": []any{}},
		},
	})
	resp, err := http.Get(ts.URL + "/boards")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["override"] != true {
		t.Errorf("custom route did not win — got %v", body)
	}
}

func TestCustomRoute204NoContent(t *testing.T) {
	ts := newFakebackWithRoutes(t, sampleDB, []routes.Route{
		{Method: "POST", Path: "/lists/{listId}/reorder", Status: 204},
	})
	resp, err := http.Post(ts.URL+"/lists/l1/reorder", "application/json", strings.NewReader(`{"order":["c1"]}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
}

func TestCustomRouteHeadersAreSet(t *testing.T) {
	ts := newFakebackWithRoutes(t, sampleDB, []routes.Route{
		{
			Method: "GET", Path: "/me", Status: 200,
			Headers: map[string]string{"X-Profile": "full"},
			Body:    map[string]any{"id": "u1", "email": "alice@example.com"},
		},
	})
	resp, err := http.Get(ts.URL + "/me")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("X-Profile"); got != "full" {
		t.Errorf("X-Profile = %q, want full", got)
	}
}

func TestRoutesLoadFromYAMLFile(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "routes.yaml")
	if err := os.WriteFile(yamlPath, []byte(`
routes:
  - method: POST
    path: /search
    status: 200
    body:
      results: []
  - method: POST
    path: /reorder
    status: 204
`), 0o644); err != nil {
		t.Fatal(err)
	}
	parsed, err := routes.LoadFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if len(parsed) != 2 {
		t.Errorf("got %d routes, want 2", len(parsed))
	}
	if parsed[0].Path != "/search" || parsed[1].Status != 204 {
		t.Errorf("parsed shape unexpected: %+v", parsed)
	}
}

func TestRoutesLoadFileMissingReturnsNil(t *testing.T) {
	out, err := routes.LoadFile("/nonexistent/routes.yaml")
	if err != nil {
		t.Errorf("missing file should return nil, got err=%v", err)
	}
	if out != nil {
		t.Errorf("missing file should return nil slice, got %v", out)
	}
}

func TestRoutesValidationRejectsMalformed(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "routes.yaml")
	if err := os.WriteFile(yamlPath, []byte(`
routes:
  - method: TELEPORT
    path: /search
`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := routes.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected validation error for unknown method")
	}
}

func TestFetchPolicyMissingHeaderReturns412(t *testing.T) {
	ts := newFakebackFull(t, sampleDB, nil, map[string]string{
		"Authorization": "Bearer ${session.jwt}",
	})
	resp, err := http.Get(ts.URL + "/boards") // no Authorization header
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPreconditionFailed {
		t.Errorf("status = %d, want 412", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["code"] != "fetchpolicy_unbound" {
		t.Errorf("code = %v, want fetchpolicy_unbound", body["code"])
	}
	if body["header"] != "Authorization" {
		t.Errorf("header = %v, want Authorization", body["header"])
	}
}

func TestFetchPolicyEmptySlotReturns412(t *testing.T) {
	ts := newFakebackFull(t, sampleDB, nil, map[string]string{
		"Authorization": "Bearer ${session.jwt}",
	})
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/boards", nil)
	req.Header.Set("Authorization", "Bearer ") // trailing slot is empty
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPreconditionFailed {
		t.Errorf("status = %d, want 412 (empty slot should fail)", resp.StatusCode)
	}
}

func TestFetchPolicyResolvedHeaderPasses(t *testing.T) {
	ts := newFakebackFull(t, sampleDB, nil, map[string]string{
		"Authorization": "Bearer ${session.jwt}",
	})
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/boards", nil)
	req.Header.Set("Authorization", "Bearer eyJhbGc.payload.sig")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 (resolved bearer should pass)", resp.StatusCode)
	}
}

func TestFetchPolicyHealthExemptFromCheck(t *testing.T) {
	// /health must respond OK even when no Authorization header is
	// declared — operator probes shouldn't need agent credentials.
	ts := newFakebackFull(t, sampleDB, nil, map[string]string{
		"Authorization": "Bearer ${session.jwt}",
	})
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 (health should be exempt)", resp.StatusCode)
	}
}

func TestFetchPolicyAdminExemptFromCheck(t *testing.T) {
	ts := newFakebackFull(t, sampleDB, nil, map[string]string{
		"Authorization": "Bearer ${session.jwt}",
	})
	resp, err := http.Post(ts.URL+"/_admin/reset", "", nil)
	if err != nil {
		t.Fatalf("POST /_admin/reset: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 (admin should be exempt)", resp.StatusCode)
	}
}

func TestFetchPolicyExactValueMatch(t *testing.T) {
	// A template without ${...} is checked for exact equality.
	ts := newFakebackFull(t, sampleDB, nil, map[string]string{
		"X-Project": "tasks-co",
	})
	// Missing
	resp, _ := http.Get(ts.URL + "/boards")
	if resp.StatusCode != http.StatusPreconditionFailed {
		t.Errorf("missing exact header: status = %d, want 412", resp.StatusCode)
	}
	resp.Body.Close()
	// Wrong value
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/boards", nil)
	req.Header.Set("X-Project", "other-co")
	resp2, _ := http.DefaultClient.Do(req)
	if resp2.StatusCode != http.StatusPreconditionFailed {
		t.Errorf("wrong exact value: status = %d, want 412", resp2.StatusCode)
	}
	resp2.Body.Close()
	// Right value
	req3, _ := http.NewRequest(http.MethodGet, ts.URL+"/boards", nil)
	req3.Header.Set("X-Project", "tasks-co")
	resp3, _ := http.DefaultClient.Do(req3)
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("right exact value: status = %d, want 200", resp3.StatusCode)
	}
	resp3.Body.Close()
}

func TestFetchPolicyConfigFromYAMLFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "expected-headers.yaml"), []byte(`
expectedHeaders:
  Authorization: "Bearer ${session.jwt}"
  X-Tenant: "${session.tenant_id}"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	expected, err := fetchpolicy.LoadFile(filepath.Join(dir, "expected-headers.yaml"))
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if len(expected) != 2 {
		t.Errorf("got %d entries, want 2", len(expected))
	}
	if expected["Authorization"] != "Bearer ${session.jwt}" {
		t.Errorf("Authorization template wrong: %v", expected["Authorization"])
	}
}

func TestScenarioBlanketErrorAppliesToAllRoutes(t *testing.T) {
	dir := scenarioDirWith(t, map[string]string{
		"auth-expired.yaml": `
name: auth-expired
error:
  status: 401
  body:
    code: auth_expired
    message: token expired
`,
	})
	reg, err := scenarios.LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.Activate("auth-expired"); err != nil {
		t.Fatal(err)
	}
	ts := newFakebackEverything(t, sampleDB, nil, nil, reg)
	resp, err := http.Get(ts.URL + "/boards")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["code"] != "auth_expired" {
		t.Errorf("code = %v, want auth_expired", body["code"])
	}
}

func TestScenarioOverrideMatchesMethodAndPath(t *testing.T) {
	dir := scenarioDirWith(t, map[string]string{
		"validation-fail.yaml": `
name: validation-fail
overrides:
  - method: POST
    path: /cards
    status: 422
    body:
      code: validation_failed
      message: title required
`,
	})
	reg, _ := scenarios.LoadDir(dir)
	_ = reg.Activate("validation-fail")
	ts := newFakebackEverything(t, sampleDB, nil, nil, reg)

	// POST /cards is overridden → 422.
	resp, _ := http.Post(ts.URL+"/cards", "application/json", strings.NewReader(`{"title":"x"}`))
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("POST /cards status = %d, want 422", resp.StatusCode)
	}
	resp.Body.Close()

	// GET /cards is NOT overridden (different method) → normal 200.
	resp2, _ := http.Get(ts.URL + "/cards")
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("GET /cards status = %d, want 200 (different method, not overridden)", resp2.StatusCode)
	}
	resp2.Body.Close()
}

func TestScenarioOverrideWithPathParam(t *testing.T) {
	dir := scenarioDirWith(t, map[string]string{
		"card-not-found.yaml": `
name: card-not-found
overrides:
  - method: GET
    path: /cards/{id}
    status: 404
    body:
      code: not_found
`,
	})
	reg, _ := scenarios.LoadDir(dir)
	_ = reg.Activate("card-not-found")
	ts := newFakebackEverything(t, sampleDB, nil, nil, reg)

	// Any GET /cards/<anything> returns 404 per the override.
	resp, _ := http.Get(ts.URL + "/cards/c1")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET /cards/c1 status = %d, want 404", resp.StatusCode)
	}
	resp.Body.Close()
	resp2, _ := http.Get(ts.URL + "/cards/zzz")
	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("GET /cards/zzz status = %d, want 404", resp2.StatusCode)
	}
	resp2.Body.Close()
}

func TestScenarioLatencyDelaysResponse(t *testing.T) {
	dir := scenarioDirWith(t, map[string]string{
		"slow.yaml": `
name: slow
latency: 150ms
`,
	})
	reg, _ := scenarios.LoadDir(dir)
	_ = reg.Activate("slow")
	ts := newFakebackEverything(t, sampleDB, nil, nil, reg)

	start := time.Now()
	resp, err := http.Get(ts.URL + "/boards")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	elapsed := time.Since(start)
	if elapsed < 150*time.Millisecond {
		t.Errorf("elapsed = %v, want >= 150ms (latency not applied)", elapsed)
	}
}

func TestScenarioPerRequestQueryOverridesActive(t *testing.T) {
	dir := scenarioDirWith(t, map[string]string{
		"slow.yaml": `
name: slow
latency: 5s
`,
		"fast.yaml": `
name: fast
`,
	})
	reg, _ := scenarios.LoadDir(dir)
	_ = reg.Activate("slow") // process default = slow

	ts := newFakebackEverything(t, sampleDB, nil, nil, reg)

	// Per-request override to "fast" should NOT delay.
	start := time.Now()
	resp, err := http.Get(ts.URL + "/boards?scenario=fast")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if time.Since(start) > 1*time.Second {
		t.Errorf("per-request fast scenario was delayed by active=slow")
	}
}

func TestScenarioUnknownNameReturns400(t *testing.T) {
	dir := scenarioDirWith(t, map[string]string{
		"fast.yaml": `name: fast`,
	})
	reg, _ := scenarios.LoadDir(dir)
	ts := newFakebackEverything(t, sampleDB, nil, nil, reg)
	resp, err := http.Get(ts.URL + "/boards?scenario=does-not-exist")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestAdminScenarioActivateAndList(t *testing.T) {
	dir := scenarioDirWith(t, map[string]string{
		"foo.yaml": `name: foo`,
		"bar.yaml": `name: bar`,
	})
	reg, _ := scenarios.LoadDir(dir)
	ts := newFakebackEverything(t, sampleDB, nil, nil, reg)

	// GET list.
	resp, err := http.Get(ts.URL + "/_admin/scenarios")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	names, _ := body["scenarios"].([]any)
	if len(names) != 2 {
		t.Errorf("got %d scenarios, want 2", len(names))
	}

	// Activate via POST.
	resp2, err := http.Post(ts.URL+"/_admin/scenarios/activate", "application/json",
		strings.NewReader(`{"name":"foo"}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("activate status = %d, want 200", resp2.StatusCode)
	}
	if reg.ActiveName() != "foo" {
		t.Errorf("active = %q, want foo", reg.ActiveName())
	}

	// Deactivate.
	resp3, _ := http.Post(ts.URL+"/_admin/scenarios/deactivate", "application/json", nil)
	resp3.Body.Close()
	if reg.ActiveName() != "" {
		t.Errorf("active after deactivate = %q, want empty", reg.ActiveName())
	}
}

func TestPersistOff_MutationsNotWrittenToDisk(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.json")
	if err := os.WriteFile(dbPath, []byte(sampleDB), 0o644); err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(dbPath)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	st, err := store.Open(store.Config{File: dbPath, IDStrategy: "int", Persist: false}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h, _ := server.New(server.Options{Logger: logger, Store: st})
	ts := httptest.NewServer(h)
	defer ts.Close()

	resp, _ := http.Post(ts.URL+"/cards", "application/json", strings.NewReader(`{"title":"x"}`))
	resp.Body.Close()

	after, _ := os.ReadFile(dbPath)
	if !bytes.Equal(original, after) {
		t.Errorf("db.json changed despite -persist=false")
	}
}
