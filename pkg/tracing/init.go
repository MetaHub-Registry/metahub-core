package tracing

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

func InitTracer(svcName, jaegerEndpoint string) (tr trace.Tracer, err error) {
	logrus.Info("Initializing Tracing")

	// create jaeger exporter
	exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(jaegerEndpoint)))
	if err != nil {
		logrus.Errorf("unable to initialize exporter due: %v", err.Error())
		return
	}
	// initialize tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(svcName),
		)),
	)
	// set tracer provider and propagator properly, this is to ensure all
	// instrumentation library could run well
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return otel.Tracer(svcName), nil
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
