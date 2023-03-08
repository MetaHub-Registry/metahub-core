package magazine

import "go.opentelemetry.io/otel/trace"

type MagConfig struct {
	RedirectDoc string
	AuthURL     string
	Tracing     TraceCfg
}

type TraceCfg struct {
	Enabled  bool
	Tracer   trace.Tracer
	Endpoint string
}
