package tavora

import (
	"encoding/json"
	"errors"
	"fmt"
)

// APIError represents an error response from the Tavora API.
//
// The server's error body is a JSON object with at least `code` and
// `message`, plus arbitrary additional fields per error class
// (e.g. `current_version` on a version_conflict). Those extras are
// captured in Details so callers can recover programmatically without
// regex-matching the human message.
type APIError struct {
	StatusCode int            `json:"-"`
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	Details    map[string]any `json:"-"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("tavora: %s (status %d)", e.Message, e.StatusCode)
}

// parseAPIError reads the response body into an APIError, populating
// Code, Message, and capturing every other top-level field into
// Details. Returns a best-effort APIError even if the body isn't JSON
// (Code/Message stay empty; the caller still gets the status).
func parseAPIError(status int, body []byte) *APIError {
	out := &APIError{StatusCode: status, Details: map[string]any{}}
	if len(body) == 0 {
		return out
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		// Non-JSON body — surface what we can without losing the status.
		return out
	}
	for k, v := range raw {
		switch k {
		case "code":
			if s, ok := v.(string); ok {
				out.Code = s
			}
		case "message":
			if s, ok := v.(string); ok {
				out.Message = s
			}
		default:
			out.Details[k] = v
		}
	}
	return out
}

// IsNotFound returns true if the error is a 404 Not Found.
func IsNotFound(err error) bool {
	var e *APIError
	if errors.As(err, &e) {
		return e.StatusCode == 404
	}
	return false
}

// IsUnauthorized returns true if the error is a 401 Unauthorized.
func IsUnauthorized(err error) bool {
	var e *APIError
	if errors.As(err, &e) {
		return e.StatusCode == 401
	}
	return false
}

// VersionConflictError is the typed representation of the server's
// `version_conflict` error returned when an UploadDocument call's
// `IfVersion` doesn't match the latest live version of (store, name).
// CurrentVersion is the version the caller should re-read against.
type VersionConflictError struct {
	*APIError
	CurrentVersion int32
}

// AsVersionConflict extracts a typed VersionConflictError from a
// returned error if and only if the server set code="version_conflict".
// The caller pattern is:
//
//	if c, ok := tavora.AsVersionConflict(err); ok {
//	    // re-read at c.CurrentVersion and retry
//	}
func AsVersionConflict(err error) (*VersionConflictError, bool) {
	var e *APIError
	if !errors.As(err, &e) {
		return nil, false
	}
	if e.Code != "version_conflict" {
		return nil, false
	}
	conflict := &VersionConflictError{APIError: e}
	if v, ok := e.Details["current_version"]; ok {
		switch n := v.(type) {
		case float64:
			conflict.CurrentVersion = int32(n)
		case int32:
			conflict.CurrentVersion = n
		case int:
			conflict.CurrentVersion = int32(n)
		}
	}
	return conflict, true
}
