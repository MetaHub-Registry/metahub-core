package tracing

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

// InitTracer initializes tracing with the legacy Jaeger collector exporter.
func InitTracer(svcName, jaegerEndpoint string) (tr trace.Tracer, err error) {
	logrus.Info("Initializing Tracing (jaeger)")

	// create jaeger exporter
	exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(jaegerEndpoint)))
	if err != nil {
		logrus.Errorf("unable to initialize exporter due: %v", err.Error())
		return
	}
	return newTracer(svcName, exporter, false), nil
}

// InitTracerOTLP initializes tracing with an OTLP/HTTP exporter (e.g. for
// VictoriaTraces). endpointURL is the full ingestion URL, for example
// http://vt.example.com/insert/opentelemetry/v1/traces. When sync is true a
// synchronous SimpleSpanProcessor is used so short-lived processes (CLI
// commands such as `metahub-admin repackage` / `compute …`) deliver their spans
// without needing an explicit flush/Shutdown — a BatchSpanProcessor would
// otherwise drop them on process exit.
func InitTracerOTLP(ctx context.Context, svcName, endpointURL string, sync bool) (tr trace.Tracer, err error) {
	logrus.Infof("Initializing Tracing (otlp/http) -> %s (sync=%v)", endpointURL, sync)
	u, err := url.Parse(endpointURL)
	if err != nil {
		logrus.Errorf("invalid OTLP endpoint URL %q: %v", endpointURL, err)
		return
	}
	// otlptracehttp v1.19.0 predates WithEndpointURL, so decompose the URL into
	// endpoint host, request path and TLS toggle.
	opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(u.Host)}
	if p := strings.TrimSpace(u.Path); p != "" {
		opts = append(opts, otlptracehttp.WithURLPath(p))
	}
	if u.Scheme != "https" {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		logrus.Errorf("unable to initialize otlp exporter due: %v", err.Error())
		return
	}
	return newTracer(svcName, exporter, sync), nil
}

// newTracer wires the given span exporter into a global TracerProvider. When
// sync is true spans are exported synchronously (SimpleSpanProcessor); otherwise
// they are batched (production default for long-running services).
func newTracer(svcName string, exporter sdktrace.SpanExporter, sync bool) trace.Tracer {
	spanOpt := sdktrace.WithBatcher(exporter)
	if sync {
		spanOpt = sdktrace.WithSyncer(exporter)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		spanOpt,
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(svcName),
		)),
	)
	// set tracer provider and propagator properly, this is to ensure all
	// instrumentation library could run well
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return otel.Tracer(svcName)
}

// Shutdown flushes remaining spans and shuts down the global tracer provider.
// Safe to call even if InitTracer was never called (no-op in that case).
func Shutdown(ctx context.Context) error {
	if tp, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); ok {
		return tp.Shutdown(ctx)
	}
	return nil
}

func SpanDebug(span trace.Span, frm string, args ...interface{}) {
	SpanLog(span, "debug", frm, args...)
}

func SpanLog(span trace.Span, level, frm string, args ...interface{}) {
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", strings.ToLower(level)),
		attribute.String("log.message", fmt.Sprintf(frm, args...)),
	))
}
