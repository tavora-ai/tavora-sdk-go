package tavora

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestAPIError_ParsesStructuredBody(t *testing.T) {
	body := []byte(`{"code":"version_conflict","message":"if_version does not match current version","current_version":7}`)

	apiErr := parseAPIError(409, body)

	if apiErr.StatusCode != 409 {
		t.Errorf("status: got %d, want 409", apiErr.StatusCode)
	}
	if apiErr.Code != "version_conflict" {
		t.Errorf("code: got %q, want version_conflict", apiErr.Code)
	}
	if apiErr.Message != "if_version does not match current version" {
		t.Errorf("message: got %q", apiErr.Message)
	}
	if v, ok := apiErr.Details["current_version"]; !ok || v != float64(7) {
		t.Errorf("details.current_version: got %v (%T), want 7", v, v)
	}
	// Built-ins must not bleed into Details.
	if _, ok := apiErr.Details["code"]; ok {
		t.Error("Details should not contain `code`")
	}
	if _, ok := apiErr.Details["message"]; ok {
		t.Error("Details should not contain `message`")
	}
}

func TestAPIError_NonJSONBody(t *testing.T) {
	apiErr := parseAPIError(502, []byte("Bad Gateway"))
	if apiErr.StatusCode != 502 {
		t.Errorf("status: got %d, want 502", apiErr.StatusCode)
	}
	if apiErr.Code != "" {
		t.Errorf("code: got %q, want empty", apiErr.Code)
	}
	if apiErr.Details == nil {
		t.Error("Details should never be nil")
	}
}

func TestAsVersionConflict(t *testing.T) {
	apiErr := parseAPIError(409, []byte(`{"code":"version_conflict","message":"...","current_version":3}`))
	conflict, ok := AsVersionConflict(apiErr)
	if !ok {
		t.Fatal("AsVersionConflict returned false on a version_conflict error")
	}
	if conflict.CurrentVersion != 3 {
		t.Errorf("CurrentVersion: got %d, want 3", conflict.CurrentVersion)
	}

	// Wrong code: not a conflict.
	other := parseAPIError(404, []byte(`{"code":"NOT_FOUND","message":"missing"}`))
	if _, ok := AsVersionConflict(other); ok {
		t.Error("AsVersionConflict matched a NOT_FOUND error")
	}

	// errors.As path — caller wraps the SDK error.
	wrapped := &APIError{StatusCode: 409, Code: "version_conflict", Details: map[string]any{"current_version": float64(5)}}
	conflict2, ok := AsVersionConflict(wrapped)
	if !ok || conflict2.CurrentVersion != 5 {
		t.Errorf("wrapped path: ok=%v, version=%d", ok, conflict2.CurrentVersion)
	}
}

func TestIsHelpers(t *testing.T) {
	notFound := parseAPIError(404, []byte(`{"message":"missing"}`))
	if !IsNotFound(notFound) {
		t.Error("IsNotFound(404) returned false")
	}
	if IsUnauthorized(notFound) {
		t.Error("IsUnauthorized(404) returned true")
	}

	unauth := parseAPIError(401, []byte(`{"message":"bad key"}`))
	if !IsUnauthorized(unauth) {
		t.Error("IsUnauthorized(401) returned false")
	}

	// Generic error: helpers must return false, not panic.
	if IsNotFound(errors.New("non-API error")) {
		t.Error("IsNotFound matched a non-APIError")
	}
}

// TestUploadDocument_VersionConflictTypedError is the end-to-end shape
// confirmation: a real SDK call against a 409 server response surfaces
// a VersionConflictError with the current_version populated. This is
// the recovery primitive central-store and any other agent-facing
// consumer needs.
func TestUploadDocument_VersionConflictTypedError(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/stores/st_1/documents", 409, map[string]any{
		"code":            "version_conflict",
		"message":         "if_version does not match current version",
		"current_version": 7,
	})

	v := int32(99)
	_, err := ts.client().UploadDocument(context.Background(), UploadDocumentInput{
		StoreID:   "st_1",
		Content:   bytes.NewReader([]byte("v100")),
		Filename:  "plan.md",
		Name:      "current_plan",
		IfVersion: &v,
	})
	if err == nil {
		t.Fatal("expected error from 409 response")
	}
	conflict, ok := AsVersionConflict(err)
	if !ok {
		t.Fatalf("expected VersionConflict, got %T: %v", err, err)
	}
	if conflict.CurrentVersion != 7 {
		t.Errorf("CurrentVersion: got %d, want 7", conflict.CurrentVersion)
	}
	if conflict.StatusCode != 409 {
		t.Errorf("StatusCode: got %d, want 409", conflict.StatusCode)
	}
}

