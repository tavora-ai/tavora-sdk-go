package tavora

import "fmt"

// APIError represents an error response from the Tavora API.
type APIError struct {
	StatusCode int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("tavora: %s (status %d)", e.Message, e.StatusCode)
}

// IsNotFound returns true if the error is a 404 Not Found.
func IsNotFound(err error) bool {
	if e, ok := err.(*APIError); ok {
		return e.StatusCode == 404
	}
	return false
}

// IsUnauthorized returns true if the error is a 401 Unauthorized.
func IsUnauthorized(err error) bool {
	if e, ok := err.(*APIError); ok {
		return e.StatusCode == 401
	}
	return false
}
