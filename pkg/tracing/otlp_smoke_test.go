package tracing

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestInitTracerOTLPConstruct exercises the OTLP tracer construction (URL
// decomposition into endpoint/path/insecure + provider wiring) without a live
// backend — otlptracehttp.New is lazy and does not dial until first export, so
// this runs offline in CI. Covers both the sync and batch processor paths.
func TestInitTracerOTLPConstruct(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		url  string
		sync bool
	}{
		{"sync-http-path", "http://vt.example.invalid/insert/opentelemetry/v1/traces", true},
		{"batch-https-noport", "https://traces.example.invalid", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr, err := InitTracerOTLP(ctx, "metahub-otlp-unit", tc.url, tc.sync)
			if err != nil {
				t.Fatalf("InitTracerOTLP(%s): %v", tc.url, err)
			}
			if tr == nil {
				t.Fatal("expected non-nil tracer")
			}
			_, span := tr.Start(ctx, "unit-span")
			span.End()
			// Shutdown flushes/settles the provider; a failed export to the
			// invalid host is swallowed by the SDK, so this must not error.
			if err := Shutdown(ctx); err != nil {
				t.Fatalf("Shutdown: %v", err)
			}
		})
	}
}

// TestInitTracerOTLPBadURL covers the url.Parse error branch.
func TestInitTracerOTLPBadURL(t *testing.T) {
	if _, err := InitTracerOTLP(context.Background(), "svc", "http://%zz", false); err == nil {
		t.Fatal("expected error for malformed endpoint URL")
	}
}

// TestOTLPSmoke is a manual smoke test that pushes a single span to an OTLP
// backend (e.g. VictoriaTraces) to validate endpoint parsing + delivery. It is
// skipped unless VT_SMOKE=1 is set, so it never runs in CI.
//
//	VT_SMOKE=1 VT_ENDPOINT=http://vt.example.com/insert/opentelemetry/v1/traces \
//	  go test -count=1 -run TestOTLPSmoke ./pkg/tracing/
func TestOTLPSmoke(t *testing.T) {
	if os.Getenv("VT_SMOKE") == "" {
		t.Skip("set VT_SMOKE=1 to run the VictoriaTraces smoke test")
	}
	endpoint := os.Getenv("VT_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://vt.example.com/insert/opentelemetry/v1/traces"
	}
	svc := os.Getenv("VT_SERVICE")
	if svc == "" {
		svc = "metahub-vt-smoke"
	}
	ctx := context.Background()
	tr, err := InitTracerOTLP(ctx, svc, endpoint, true)
	if err != nil {
		t.Fatalf("InitTracerOTLP: %v", err)
	}
	_, span := tr.Start(ctx, "smoke-span")
	span.AddEvent("smoke event")
	time.Sleep(2 * time.Millisecond)
	span.End()
	if err := Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	t.Logf("sent smoke span to %s as service %q", endpoint, svc)
}
