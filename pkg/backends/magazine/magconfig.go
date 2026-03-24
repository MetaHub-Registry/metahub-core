package magazine

import "go.opentelemetry.io/otel/trace"

type MagConfig struct {
	RedirectDoc string
	AuthURL     string
	Tracing     TraceCfg
	Args        map[string]string
	OAuth       OAuthCfg
	OIDC        OIDCCfg
	CORS        CORS
}

type OIDCCfg struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

type CORS struct {
	Enabled bool
}
type OAuthCfg struct {
	Enabled      bool
	ClientID     string
	ClientSecret string
	StateString  string
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
