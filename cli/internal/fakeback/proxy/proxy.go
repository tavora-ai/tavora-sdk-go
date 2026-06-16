// Package proxy forwards requests fakeback can't satisfy locally
// (no matching custom route, no matching CRUD resource, no scenario
// override) to an upstream HTTP server. Paired with the recorder
// package it captures every proxied response into a file the author
// promotes into routes.yaml or db.json — the canonical record-mode
// path for bootstrapping a mock from a real backend.
//
// The proxy strips a few hop-by-hop headers and refuses to propagate
// X-Tavora-Fake-Backend (the upstream is real; we shouldn't tell it
// we're a fake), then mirrors the upstream's status / headers / body
// back to the caller verbatim. The recorder, if attached, observes
// the same response after it's been sent.
package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Captured is the per-request snapshot the recorder ingests. The
// proxy populates it inline with the response; the recorder writes it
// to disk asynchronously.
type Captured struct {
	Method  string
	Path    string
	Query   string
	Status  int
	Headers map[string]string
	// Body is the response body bytes; UTF-8 JSON when the response
	// is JSON, raw bytes otherwise. The recorder decides whether to
	// inline-decode or store as base64 / raw string.
	Body []byte
}

// RecorderFunc is the optional callback fakeback invokes after a
// proxied response is sent. The recorder package implements this.
type RecorderFunc func(Captured)

// Handler is the http.Handler that forwards to upstream. Construct
// with New().
type Handler struct {
	upstream *url.URL
	record   RecorderFunc
	timeout  time.Duration
}

// New builds a Handler. upstream must be an absolute URL (scheme +
// host); record may be nil for proxy-without-capture. Returns an
// error if upstream is malformed.
func New(upstream string, record RecorderFunc) (*Handler, error) {
	u, err := url.Parse(upstream)
	if err != nil {
		return nil, fmt.Errorf("proxy: parse upstream: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("proxy: upstream must include scheme + host, got %q", upstream)
	}
	return &Handler{
		upstream: u,
		record:   record,
		timeout:  30 * time.Second,
	}, nil
}

// ServeHTTP proxies r to the upstream, mirrors the response, and (if
// a recorder is attached) emits a Captured snapshot.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	target := *h.upstream
	target.Path = singleJoinPath(target.Path, r.URL.Path)
	target.RawQuery = r.URL.RawQuery

	// Build the outbound request. Drain the inbound body once since
	// http.NewRequest re-reads it.
	body, _ := io.ReadAll(r.Body)
	r.Body.Close()

	outReq, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), bytes.NewReader(body))
	if err != nil {
		writeProxyError(w, fmt.Sprintf("build upstream request: %v", err))
		return
	}
	copyRequestHeaders(outReq.Header, r.Header)
	outReq.Host = h.upstream.Host

	client := &http.Client{Timeout: h.timeout}
	resp, err := client.Do(outReq)
	if err != nil {
		writeProxyError(w, fmt.Sprintf("upstream call failed: %v", err))
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20)) // 4 MiB cap

	// Copy upstream headers minus hop-by-hop ones; the X-Tavora-Mock /
	// X-Tavora-Fake-Backend headers from the upstream (if any) are
	// dropped — the fakeback middleware stamps its own.
	for k, vs := range resp.Header {
		if isHopByHop(k) || strings.EqualFold(k, "X-Tavora-Mock") || strings.EqualFold(k, "X-Tavora-Fake-Backend") {
			continue
		}
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)

	if h.record != nil {
		hdrs := make(map[string]string, len(resp.Header))
		for k := range resp.Header {
			if isHopByHop(k) {
				continue
			}
			hdrs[k] = resp.Header.Get(k)
		}
		h.record(Captured{
			Method:  r.Method,
			Path:    r.URL.Path,
			Query:   r.URL.RawQuery,
			Status:  resp.StatusCode,
			Headers: hdrs,
			Body:    respBody,
		})
	}
}

// copyRequestHeaders forwards the inbound request headers minus
// hop-by-hop ones. The Authorization header IS forwarded so the
// agent's fetchPolicies-injected bearer reaches the upstream during
// recording.
func copyRequestHeaders(dst, src http.Header) {
	for k, vs := range src {
		if isHopByHop(k) {
			continue
		}
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
}

var hopByHop = map[string]struct{}{
	"connection":          {},
	"keep-alive":          {},
	"proxy-authenticate":  {},
	"proxy-authorization": {},
	"te":                  {},
	"trailers":            {},
	"transfer-encoding":   {},
	"upgrade":             {},
}

func isHopByHop(name string) bool {
	_, ok := hopByHop[strings.ToLower(name)]
	return ok
}

// singleJoinPath joins a + b with a single slash separator, matching
// httputil.NewSingleHostReverseProxy's behavior.
func singleJoinPath(a, b string) string {
	aSlash := strings.HasSuffix(a, "/")
	bSlash := strings.HasPrefix(b, "/")
	switch {
	case aSlash && bSlash:
		return a + b[1:]
	case !aSlash && !bSlash:
		return a + "/" + b
	}
	return a + b
}

func writeProxyError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusBadGateway)
	fmt.Fprintf(w, `{"error":%q,"status":502,"code":"upstream_unreachable"}`, msg)
}
