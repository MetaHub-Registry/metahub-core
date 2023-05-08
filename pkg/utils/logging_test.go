package utils

import (
	"fmt"
	"net/http/httptest"
	"testing"
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
