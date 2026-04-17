package magazine

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

// RegistryConfig holds per-registry configuration for upstream pullthrough.
// MVP scope: anonymous-pull registries only (e.g., zot.example.com).
// Authenticated registries (docker.io, ghcr.io) are deferred to a follow-up change.
//
// Name is the slug alias used in image paths (e.g. "local-reg"). Clients pull
// via <metahub>/<name>/<repo>:<tag>. It must be a valid OCI path segment (no
// colons allowed — ports trip up reference parsers).
//
// URL is the actual network endpoint of the upstream registry (e.g.
// "registry:5050"). When empty, Name is used as both the slug and the
// endpoint (backward compatible for registries like "zot.example.com" that
// are valid as both a slug and a hostname).
//
// Legacy field "hostname" in YAML is read as Name for backward compatibility.
type RegistryConfig struct {
	Name      string `yaml:"name" json:"name"`
	URL       string `yaml:"url" json:"url,omitempty"`
	AllowAnon bool   `yaml:"allow_anon" json:"allowAnon"`
	TLS       bool   `yaml:"tls" json:"tls"`

	// Hostname is kept for backward compatibility with existing YAML configs.
	// When Name is empty but Hostname is set, Name is populated from Hostname.
	// New configs should use "name" instead.
	Hostname string `yaml:"hostname" json:"hostname,omitempty"`
}

// EffectiveURL returns the network endpoint to use for upstream connections.
// If URL is set, it is returned. Otherwise EffectiveName is used (backward
// compat for registries where the slug and the hostname are the same).
func (rc RegistryConfig) EffectiveURL() string {
	if rc.URL != "" {
		return rc.URL
	}
	return rc.EffectiveName()
}

// EffectiveName returns the slug alias used in image paths. Handles the
// legacy Hostname field: if Name is empty but Hostname is set, Hostname
// is returned.
func (rc RegistryConfig) EffectiveName() string {
	if rc.Name != "" {
		return rc.Name
	}
	return rc.Hostname
}

// normalize fills Name from Hostname if needed (legacy compat) and ensures
// the config is internally consistent. Called by config loaders after parsing.
func (rc *RegistryConfig) normalize() {
	if rc.Name == "" && rc.Hostname != "" {
		rc.Name = rc.Hostname
	}
	// Keep Hostname in sync for JSON serialisation backward compat
	if rc.Hostname == "" && rc.Name != "" {
		rc.Hostname = rc.Name
	}
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
// a configured upstream registry name (slug). If so, it returns the registry
// config's EffectiveURL (for upstream connections) and the remaining repo path
// (stripped of the slug prefix). If no match is found, found is false and
// strippedRepo equals the original repoPath.
//
// A single-segment repo (no "/" separator) can never match because there would
// be nothing left as the repository name after stripping the prefix.
func (mag Magazine) ResolveUpstreamRegistry(repoPath string) (regURL, strippedRepo string, found bool) {
	idx := strings.Index(repoPath, "/")
	if idx < 0 {
		// Single segment — cannot be registry/repo
		return "", repoPath, false
	}
	firstSeg := repoPath[:idx]
	rest := repoPath[idx+1:]
	for _, reg := range mag.Config.Registries {
		if reg.EffectiveName() == firstSeg {
			return reg.EffectiveURL(), rest, true
		}
	}
	return "", repoPath, false
}

// ValidateRegistries checks the registries list for obvious configuration
// errors (e.g. empty names caused by YAML parsing issues). Returns a slice
// of errors — one per invalid entry. An empty return slice means all entries
// are valid.
func ValidateRegistries(regs []RegistryConfig) []error {
	var errs []error
	for i, reg := range regs {
		if reg.EffectiveName() == "" {
			errs = append(errs, fmt.Errorf("registries[%d]: empty name — set 'name' (or legacy 'hostname') in the config", i))
		}
	}
	return errs
}

// RegistryConfigFromHostname creates a RegistryConfig from a plain hostname
// string with sensible defaults: AllowAnon true (MVP assumes anonymous
// registries), TLS inferred from the hostname pattern. Both Name and URL
// are set to the hostname (backward compat for legacy string configs).
func RegistryConfigFromHostname(hostname string) RegistryConfig {
	rc := RegistryConfig{
		Name:      hostname,
		Hostname:  hostname,
		AllowAnon: true,
		TLS:       inferTLS(hostname),
	}
	return rc
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
	if v, ok := m["name"]; ok {
		if s, ok := v.(string); ok {
			rc.Name = s
		}
	}
	if v, ok := m["hostname"]; ok {
		if s, ok := v.(string); ok {
			rc.Hostname = s
		}
	}
	if v, ok := m["url"]; ok {
		if s, ok := v.(string); ok {
			rc.URL = s
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
	rc.normalize()
	return rc
}

// registryConfigFromStringMap converts a map[string]interface{} into a
// RegistryConfig. Some YAML libraries produce this variant.
func registryConfigFromStringMap(m map[string]interface{}) RegistryConfig {
	rc := RegistryConfig{TLS: true, AllowAnon: true}
	if v, ok := m["name"]; ok {
		if s, ok := v.(string); ok {
			rc.Name = s
		}
	}
	if v, ok := m["hostname"]; ok {
		if s, ok := v.(string); ok {
			rc.Hostname = s
		}
	}
	if v, ok := m["url"]; ok {
		if s, ok := v.(string); ok {
			rc.URL = s
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
	rc.normalize()
	return rc
}
