package magazine

import "go.opentelemetry.io/otel/trace"

type MagConfig struct {
	RedirectDoc string
	AuthURL     string
	Tracing     TraceCfg
	Args        map[string]string
}

type TraceCfg struct {
	Enabled  bool
	Tracer   trace.Tracer
	Endpoint string
}

func (mag Magazine) GetConfig(k string) (v string, ok bool) {
	v, ok = mag.Config.Args[k]
	return
}
