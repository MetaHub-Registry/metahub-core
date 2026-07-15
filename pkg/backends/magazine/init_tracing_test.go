package magazine

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"

	"github.com/metahub-registry/metahub-core/pkg/tracing"
)

// TestInitTracingProtocols covers initTracing's OTLP and Jaeger branches
// (including the tracing.* env binding) without a live backend. The throwaway
// exporters target an invalid host (lazy dial, so no error at construction),
// and the global TracerProvider is restored afterwards so the rest of the suite
// is not redirected to these dead exporters.
func TestInitTracingProtocols(t *testing.T) {
	orig := otel.GetTracerProvider()
	defer otel.SetTracerProvider(orig)
	defer func() {
		viper.Set("tracing.endpoint", "")
		viper.Set("tracing.name", "")
		viper.Set("tracing.protocol", "")
		viper.Set("tracing.sync", false)
	}()

	for _, proto := range []string{"otlp", "jaeger"} {
		viper.Set("tracing.endpoint", "http://trace.example.invalid/insert/opentelemetry/v1/traces")
		viper.Set("tracing.name", "unit-"+proto)
		viper.Set("tracing.protocol", proto)
		viper.Set("tracing.sync", false)
		cmd := &cobra.Command{}
		cfg := &MagConfig{}
		if _, err := initTracing(cmd, cfg); err != nil {
			t.Fatalf("initTracing(%s): %v", proto, err)
		}
		if !cfg.Tracing.Enabled || cfg.Tracing.Endpoint == "" {
			t.Fatalf("proto %s: expected tracing enabled with endpoint", proto)
		}
		// Stop the throwaway provider's background workers before the next case.
		_ = tracing.Shutdown(context.Background())
	}
}

// TestInitTracingDisabled covers the disabled path (empty endpoint → no tracer).
func TestInitTracingDisabled(t *testing.T) {
	orig := otel.GetTracerProvider()
	defer otel.SetTracerProvider(orig)
	viper.Set("tracing.endpoint", "")
	viper.Set("tracing.name", "")
	cmd := &cobra.Command{}
	cfg := &MagConfig{}
	if _, err := initTracing(cmd, cfg); err != nil {
		t.Fatalf("initTracing(disabled): %v", err)
	}
	if cfg.Tracing.Enabled {
		t.Fatal("expected tracing disabled when endpoint empty")
	}
}
