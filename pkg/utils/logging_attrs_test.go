package utils

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
)

// TestInitLoggingAttributes exercises the attribute type switch (int, string
// and the default branch) so the logging helper is covered independently of
// the database tests.
func TestInitLoggingAttributes(t *testing.T) {
	tracer := otel.Tracer("test")
	_, _, span := InitLogging(context.Background(), tracer, map[string]any{
		"package": "utils",
		"func":    "TestInitLoggingAttributes",
		"count":   1,
		"label":   "value",
		"ratio":   1.5,
	})
	span.End()
}
