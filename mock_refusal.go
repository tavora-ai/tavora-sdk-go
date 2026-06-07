package tavora

import (
	"errors"
	"net/http"
	"os"
)

// mockResponseHeader is the response header that `tavora-mock` stamps
// on every response. The SDK refuses to operate against a server that
// sets it unless the operator opts in via the TAVORA_ALLOW_MOCK
// environment variable.
//
// Rationale: the mock is dev-only and returns canned data. Pointing a
// production CLI or app at the mock by accident would silently
// "succeed" with fake state — the explicit refusal turns that mistake
// into a loud error at the first response. CI and the mock binary's
// own launcher are expected to set TAVORA_ALLOW_MOCK=1.
const mockResponseHeader = "X-Tavora-Mock"

// allowMockEnv is the environment variable that opts a caller in to
// using a mock server. Any value other than "1" / "true" / "yes" is
// treated as not-set.
const allowMockEnv = "TAVORA_ALLOW_MOCK"

// ErrMockRefused is returned when the SDK sees X-Tavora-Mock: true on
// a response and TAVORA_ALLOW_MOCK is not set. Callable code can use
// errors.Is to differentiate this from genuine API errors.
var ErrMockRefused = errors.New(
	"tavora: refusing to operate against a mock server " +
		"(set TAVORA_ALLOW_MOCK=1 to override — only valid in tests/CI)",
)

// checkMockResponse returns ErrMockRefused if resp is from a server
// that identifies as a mock (X-Tavora-Mock: true) and the operator
// hasn't set TAVORA_ALLOW_MOCK=1. Returns nil otherwise. Safe to
// call on a nil header map.
func checkMockResponse(h http.Header) error {
	if h == nil {
		return nil
	}
	if h.Get(mockResponseHeader) != "true" {
		return nil
	}
	if mockAllowed() {
		return nil
	}
	return ErrMockRefused
}

func mockAllowed() bool {
	switch os.Getenv(allowMockEnv) {
	case "1", "true", "yes", "TRUE", "YES":
		return true
	}
	return false
}
