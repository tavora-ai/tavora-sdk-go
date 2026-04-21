package tavora

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// recordedRequest captures what the SDK sent to the server.
type recordedRequest struct {
	Method string
	Path   string
	Body   string
	Header http.Header
}

// testServer is a lightweight httptest wrapper that records requests and
// returns canned responses configured per-path.
type testServer struct {
	*httptest.Server

	mu       sync.Mutex
	routes   map[string]testRoute
	requests []recordedRequest
}

type testRoute struct {
	status int
	body   interface{}
	raw    string // if non-empty, written verbatim (for SSE)
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	ts := &testServer{
		routes: make(map[string]testRoute),
	}
	ts.Server = httptest.NewServer(http.HandlerFunc(ts.handler))
	t.Cleanup(ts.Close)
	return ts
}

// on registers a canned JSON response for a method+path pair.
func (ts *testServer) on(method, path string, status int, body interface{}) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.routes[method+" "+path] = testRoute{status: status, body: body}
}

// onRaw registers a raw (non-JSON) response for SSE streams etc.
func (ts *testServer) onRaw(method, path string, status int, raw string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.routes[method+" "+path] = testRoute{status: status, raw: raw}
}

func (ts *testServer) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	ts.mu.Lock()
	ts.requests = append(ts.requests, recordedRequest{
		Method: r.Method,
		Path:   r.RequestURI,
		Body:   string(body),
		Header: r.Header.Clone(),
	})
	route, ok := ts.routes[r.Method+" "+r.URL.Path]
	ts.mu.Unlock()

	if !ok {
		http.Error(w, "no route registered", http.StatusNotFound)
		return
	}

	if route.raw != "" {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(route.status)
		w.Write([]byte(route.raw))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(route.status)
	json.NewEncoder(w).Encode(route.body)
}

// lastRequest returns the most recent recorded request.
func (ts *testServer) lastRequest(t *testing.T) recordedRequest {
	t.Helper()
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if len(ts.requests) == 0 {
		t.Fatal("no requests recorded")
	}
	return ts.requests[len(ts.requests)-1]
}

// requestCount returns the number of requests recorded.
func (ts *testServer) requestCount() int {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return len(ts.requests)
}

// client returns an SDK client pointing at this test server.
func (ts *testServer) client() *Client {
	return NewClient(ts.URL, "tvr_testkey")
}

// assertEqual is a lightweight assertion helper.
func assertEqual(t *testing.T, label string, got, want interface{}) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", label, got, want)
	}
}

// assertNoError fails the test if err is non-nil.
func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// assertError fails the test if err is nil.
func assertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
