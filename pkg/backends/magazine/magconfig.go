package magazine

import (
	"strings"

	"go.opentelemetry.io/otel/trace"
)

// RegistryConfig holds per-registry configuration for upstream pullthrough.
// MVP scope: anonymous-pull registries only (e.g., zot.example.com).
// Authenticated registries (docker.io, ghcr.io) are deferred to a follow-up change.
type RegistryConfig struct {
	Hostname  string `yaml:"hostname" json:"hostname"`
	AllowAnon bool   `yaml:"allow_anon" json:"allowAnon"`
	TLS       bool   `yaml:"tls" json:"tls"`
}

type MagConfig struct {
	RedirectDoc string
	AuthURL     string
	Tracing     TraceCfg
	Args        map[string]string
	Registries  []RegistryConfig
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

// GetRegistries returns the configured upstream registries.
func (mag Magazine) GetRegistries() []RegistryConfig {
	return mag.Config.Registries
}

// ResolveUpstreamRegistry checks if the first path segment of repoPath matches
// a configured upstream registry hostname. If so, it returns the hostname and
// the remaining repo path (stripped of the prefix). If no match is found,
// found is false and strippedRepo equals the original repoPath.
//
// A single-segment repo (no "/" separator) can never match because there would
// be nothing left as the repository name after stripping the prefix.
func (mag Magazine) ResolveUpstreamRegistry(repoPath string) (regHostname, strippedRepo string, found bool) {
	idx := strings.Index(repoPath, "/")
	if idx < 0 {
		// Single segment — cannot be registry/repo
		return "", repoPath, false
	}
	firstSeg := repoPath[:idx]
	rest := repoPath[idx+1:]
	for _, reg := range mag.Config.Registries {
		if reg.Hostname == firstSeg {
			return reg.Hostname, rest, true
		}
	}
	return "", repoPath, false
}

// RegistryConfigFromHostname creates a RegistryConfig from a plain hostname
// string with sensible defaults: AllowAnon true (MVP assumes anonymous
// registries), TLS inferred from the hostname pattern.
func RegistryConfigFromHostname(hostname string) RegistryConfig {
	return RegistryConfig{
		Hostname:  hostname,
		AllowAnon: true,
		TLS:       inferTLS(hostname),
	}
}

// inferTLS returns false for local/test registries (localhost, 172.*, *:5050),
// true for everything else. Matches the existing NewRegClient TLS logic.
func inferTLS(hostname string) bool {
	switch {
	case len(hostname) >= 9 && hostname[:9] == "localhost":
		return false
	case len(hostname) >= 4 && hostname[:4] == "172.":
		return false
	case hostname == "registry:5050":
		return false
	default:
		return true
	}
}

// registryConfigFromMap converts a map[interface{}]interface{} (viper YAML)
// into a RegistryConfig.
func registryConfigFromMap(m map[interface{}]interface{}) RegistryConfig {
	rc := RegistryConfig{TLS: true, AllowAnon: true}
	if v, ok := m["hostname"]; ok {
		if s, ok := v.(string); ok {
			rc.Hostname = s
		}
	}
	if v, ok := m["allow_anon"]; ok {
		if b, ok := v.(bool); ok {
			rc.AllowAnon = b
		}
	}
	if v, ok := m["tls"]; ok {
		if b, ok := v.(bool); ok {
			rc.TLS = b
		}
	}
	return rc
}

// registryConfigFromStringMap converts a map[string]interface{} into a
// RegistryConfig. Some YAML libraries produce this variant.
func registryConfigFromStringMap(m map[string]interface{}) RegistryConfig {
	rc := RegistryConfig{TLS: true, AllowAnon: true}
	if v, ok := m["hostname"]; ok {
		if s, ok := v.(string); ok {
			rc.Hostname = s
		}
	}
	if v, ok := m["allow_anon"]; ok {
		if b, ok := v.(bool); ok {
			rc.AllowAnon = b
		}
	}
	if v, ok := m["tls"]; ok {
		if b, ok := v.(bool); ok {
			rc.TLS = b
		}
	}
	return rc
}
