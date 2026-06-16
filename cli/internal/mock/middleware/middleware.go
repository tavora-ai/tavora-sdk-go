// Package middleware re-exports the shared HTTP middlewares from
// internal/mockcommon/middleware and adds the tavora-mock-specific
// MockHeader middleware.
//
// The middlewares moved to mockcommon during v0.1 of tavora-fake-backend
// so both forks could share the request-id / logger / recoverer
// scaffolding. Existing callers in this package keep the same import
// path and identifiers — only the implementation moved.
package middleware

import (
	"context"
	"net/http"

	common "github.com/tavora-ai/tavora-sdk-go/cli/internal/mockcommon/middleware"
)

// MockHeaderName is the response header that identifies a mock server.
// Clients (the Go SDK) check for this and refuse to operate unless
// TAVORA_ALLOW_MOCK=1 is set in the environment.
const MockHeaderName = "X-Tavora-Mock"

// MockHeader adds `X-Tavora-Mock: true` to every response.
func MockHeader(next http.Handler) http.Handler {
	return common.SelfIDHeader(MockHeaderName, "true")(next)
}

// RequestID attaches a request id and echoes it on the response.
var RequestID = common.RequestID

// RequestIDFrom returns the id attached by RequestID, or "".
func RequestIDFrom(ctx context.Context) string {
	return common.RequestIDFrom(ctx)
}

// Logger logs one structured line per request.
var Logger = common.Logger

// Recoverer catches panics, logs them, and returns 500.
var Recoverer = common.Recoverer
