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
	ErrorHandler_WithWWW(w, nil, nil, "repo", "action", e, "DENIED", "ErrorHandler_WithWWW")
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
