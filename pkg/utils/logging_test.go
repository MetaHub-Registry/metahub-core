package utils

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"gotest.tools/v3/assert"
)

func TestRunNIL(t *testing.T) {
	e := fmt.Errorf("test")
	w := httptest.NewRecorder()
	ErrorHandler(nil, nil, e, "ErrorHandler")
	ErrorHandlerWithWWW(w, nil, nil, "repo", "action", e, "DENIED", "ErrorHandlerWithWWW")
	NewError(nil, nil, "NewError")
	Debug(nil, nil, "Debug")
	DebugOK(nil, nil, "Debug")
	Warn(nil, nil, "Warn")
	Info(nil, nil, "Info")
}

func TestErrTypeHTTPStatus(t *testing.T) {
	w := httptest.NewRecorder()
	ErrTypeToHTTPStatus(w, "BAD_REQUEST")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	w = httptest.NewRecorder()
	ErrTypeToHTTPStatus(w, "UNAUTHORIZED")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	w = httptest.NewRecorder()
	ErrTypeToHTTPStatus(w, "FORBIDDEN")
	assert.Equal(t, http.StatusForbidden, w.Code)
	w = httptest.NewRecorder()
	ErrTypeToHTTPStatus(w, "NOT_FOUND")
	assert.Equal(t, http.StatusNotFound, w.Code)
	w = httptest.NewRecorder()
	ErrTypeToHTTPStatus(w, "INTERNAL_ERROR")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	w = httptest.NewRecorder()
	ErrTypeToHTTPStatus(w, "UNKNOWN")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestNewErrorPlainWWW(t *testing.T) {
	w := httptest.NewRecorder()
	err := NewErrorPlainWWW(w, nil, nil, "BAD_REQUEST", "NewErrorPlainWWW")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.ErrorContains(t, err, "NewErrorPlainWWW")
}

func TestErrHandlerPlainWWW(t *testing.T) {
	w := httptest.NewRecorder()
	err := ErrorHandlerPlainWWW(w, nil, nil, fmt.Errorf("test"), "BAD_REQUEST", "ErrorHandlerPlainWWW")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.ErrorContains(t, err, "ErrorHandlerPlainWWW")

}

// OCI error codes must carry their spec status. DENIED and UNAUTHORIZED used to
// fall through to the default and return 500 with an "Unknown error type"
// warning, so an authorization refusal was indistinguishable from a crash --
// a quota rejection, a missing profile and a namespace-authz denial all showed
// up as HTTP 500 while debugging the same feature.
func TestErrorHandlerWithWWWOCIStatus(t *testing.T) {
	e := fmt.Errorf("boom")
	for _, tc := range []struct {
		eType string
		want  int
	}{
		{"MANIFEST_UNKNOWN", http.StatusNotFound},
		{"MANIFEST_INVALID", http.StatusBadRequest},
		{"INTERNAL_ERROR", http.StatusBadRequest},
		{"DENIED", http.StatusForbidden},
		{"UNAUTHORIZED", http.StatusUnauthorized},
		{"SOMETHING_UNMAPPED", http.StatusInternalServerError},
	} {
		w := httptest.NewRecorder()
		ErrorHandlerWithWWW(w, nil, nil, "repo", "push", e, tc.eType, "")
		assert.Equal(t, w.Code, tc.want)
	}
}
