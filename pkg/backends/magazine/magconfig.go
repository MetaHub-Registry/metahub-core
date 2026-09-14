package magazine

import (
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// PullThroughConfig is the nested `pull_through` block on a RegistryConfig
// (session readd-pull-through-cache, decision D1). The zero value means the
// registry is not a pull-through candidate: only registries with Enabled true
// participate. Legacy hostname-derived registries (RegistryConfigFromHostname)
// default to Enabled true with Cache "off", so existing deployments keep
// working without adding a block (D3).
type PullThroughConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	// ClientRedirect allows metahub-api to 307 the client to the upstream blob
	// URL instead of proxying. Ignored for authenticated upstreams.
	ClientRedirect bool `yaml:"client_redirect" json:"clientRedirect"`
	// Cache is "off" | "async" (D2). Defaults to "async" when enabled.
	Cache string `yaml:"cache" json:"cache"`
	// RefreshInterval is the manifest TTL; 0 means always re-check.
	RefreshInterval time.Duration `yaml:"refresh_interval" json:"refreshInterval"`
	// MaxCacheBytes is the per-registry cache soft cap; 0 means unlimited.
	MaxCacheBytes int64 `yaml:"max_cache_bytes" json:"maxCacheBytes"`
	// Workers overrides the global worker-pool size; 0 means global default.
	Workers int `yaml:"workers" json:"workers"`
}

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
	// User and Password enable basic-auth pull-through for private registries.
	// A registry with credentials does not need AllowAnon=true to be pull-through
	// eligible (it authenticates instead of pulling anonymously).
	User     string `yaml:"user" json:"user,omitempty"`
	Password string `yaml:"password" json:"password,omitempty"`

	// PullThrough is the nested pull-through opt-in/behaviour block (D1).
	PullThrough PullThroughConfig `yaml:"pull_through" json:"pullThrough"`

	// Hostname is kept for backward compatibility with existing YAML configs.
	// When Name is empty but Hostname is set, Name is populated from Hostname.
	// New configs should use "name" instead.
	Hostname string `yaml:"hostname" json:"hostname,omitempty"`
}

// hasCredentials reports whether basic-auth credentials are configured.
func (rc RegistryConfig) hasCredentials() bool {
	return rc.User != "" && rc.Password != ""
}

// PullThroughEligible reports whether a registry may serve as a pull-through
// upstream. Requires pull_through.enabled plus either AllowAnon (anonymous
// pull) or configured credentials (basic-auth pull).
func (rc RegistryConfig) PullThroughEligible() bool {
	return rc.PullThrough.Enabled && (rc.AllowAnon || rc.hasCredentials())
}

// validate rejects an unknown cache mode at load time (D2).
func (p PullThroughConfig) validate() error {
	switch p.Cache {
	case "", "off", "async":
		return nil
	}
	return fmt.Errorf("pull_through.cache must be 'off' or 'async' (got %q)", p.Cache)
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
	// LogicalPoolBytes is the no-overcommit logical storage-pool budget
	// (add-registry-project-quotas): Σ(project quota) may not exceed it.
	// 0 means unlimited (no pool cap enforced).
	LogicalPoolBytes int64
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
		// Only opt-in, anonymous registries participate (D3); legacy
		// hostname-derived entries are enabled at construction.
		if !reg.PullThroughEligible() {
			continue
		}
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
		// Credential pairing (flat user/password tier): both keys must be
		// set together — exactly one of the two means a typo'd config that
		// would silently authenticate anonymously.
		if (reg.User == "") != (reg.Password == "") {
			errs = append(errs, fmt.Errorf("registries[%d]: incomplete credentials — 'user' and 'password' must be set together", i))
		}
		if e := reg.PullThrough.validate(); e != nil {
			errs = append(errs, fmt.Errorf("registries[%d]: %w", i, e))
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
		// Legacy entries (--registries, comma-string config, plain-string YAML
		// entries) stay pull-through-capable with caching off (D3).
		PullThrough: PullThroughConfig{Enabled: true, Cache: "off"},
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

// asStringMap normalises the two map shapes viper/yaml produce.
func asStringMap(v interface{}) map[string]interface{} {
	switch m := v.(type) {
	case map[string]interface{}:
		return m
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(m))
		for k, val := range m {
			if ks, ok := k.(string); ok {
				out[ks] = val
			}
		}
		return out
	default:
		return nil
	}
}

// parseDurationValue accepts a Go duration string ("1h") or a numeric value in
// seconds.
func parseDurationValue(v interface{}) (time.Duration, bool) {
	switch d := v.(type) {
	case string:
		if d == "" {
			return 0, false
		}
		parsed, err := time.ParseDuration(d)
		if err != nil {
			return 0, false
		}
		return parsed, true
	case int:
		return time.Duration(d) * time.Second, true
	case int64:
		return time.Duration(d) * time.Second, true
	case float64:
		return time.Duration(d * float64(time.Second)), true
	default:
		return 0, false
	}
}

// toInt64Value coerces the numeric shapes yaml produces.
func toInt64Value(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		return int64(n), true
	default:
		return 0, false
	}
}

// toIntValue coerces the numeric shapes yaml produces.
func toIntValue(v interface{}) (int, bool) {
	n, ok := toInt64Value(v)
	return int(n), ok
}

// pullThroughFromMap parses the nested pull_through block and applies the D2
// defaults. An absent block yields the zero value (disabled).
func pullThroughFromMap(v interface{}) PullThroughConfig {
	var p PullThroughConfig
	m := asStringMap(v)
	if m == nil {
		return p
	}
	if b, ok := m["enabled"].(bool); ok {
		p.Enabled = b
	}
	if b, ok := m["client_redirect"].(bool); ok {
		p.ClientRedirect = b
	}
	if s, ok := m["cache"].(string); ok {
		p.Cache = s
	}
	if d, ok := parseDurationValue(m["refresh_interval"]); ok {
		p.RefreshInterval = d
	}
	if n, ok := toInt64Value(m["max_cache_bytes"]); ok {
		p.MaxCacheBytes = n
	}
	if n, ok := toIntValue(m["workers"]); ok {
		p.Workers = n
	}
	// Defaults apply only when enabled and the key was omitted (an explicit
	// refresh_interval: 0 means "always re-check", D1).
	if p.Enabled {
		if _, present := m["cache"]; !present {
			p.Cache = "async"
		}
		if _, present := m["refresh_interval"]; !present {
			p.RefreshInterval = time.Hour
		}
	}
	return p
}

// strValue coerces the string shapes yaml produces; absent keys or keys of
// any other type yield "".
func strValue(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// boolValue coerces the boolean shapes yaml produces; absent keys or keys of
// any other type yield def.
func boolValue(v interface{}, def bool) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return def
}

// registryConfigFromKV builds a RegistryConfig from a parsed YAML registry
// entry. Missing tls/allow_anon keys keep the defaults (TLS on, anon pulls
// allowed); keys holding the wrong type are ignored exactly like missing ones.
func registryConfigFromKV(m map[string]interface{}) RegistryConfig {
	rc := RegistryConfig{
		TLS:       boolValue(m["tls"], true),
		AllowAnon: boolValue(m["allow_anon"], true),
	}
	rc.Name = strValue(m["name"])
	rc.Hostname = strValue(m["hostname"])
	rc.URL = strValue(m["url"])
	rc.User = strValue(m["user"])
	rc.Password = strValue(m["password"])
	rc.PullThrough = pullThroughFromMap(m["pull_through"])
	rc.normalize()
	return rc
}

// registryConfigFromMap converts a map[interface{}]interface{} (viper YAML)
// into a RegistryConfig.
func registryConfigFromMap(m map[interface{}]interface{}) RegistryConfig {
	return registryConfigFromKV(asStringMap(m))
}

// registryConfigFromStringMap converts a map[string]interface{} into a
// RegistryConfig. Some YAML libraries produce this variant.
func registryConfigFromStringMap(m map[string]interface{}) RegistryConfig {
	return registryConfigFromKV(m)
}
