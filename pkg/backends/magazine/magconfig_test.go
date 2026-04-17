package magazine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func TestRegistryConfig_JSONSerialization(t *testing.T) {
	rc := RegistryConfig{
		Name:      "zot.example.com",
		Hostname:  "zot.example.com",
		AllowAnon: true,
		TLS:       true,
	}
	b, err := json.Marshal(rc)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"name":"zot.example.com"`)
	assert.Contains(t, string(b), `"allowAnon":true`)
	assert.Contains(t, string(b), `"tls":true`)

	var rc2 RegistryConfig
	err = json.Unmarshal(b, &rc2)
	require.NoError(t, err)
	assert.Equal(t, rc.Name, rc2.Name)
	assert.Equal(t, rc.AllowAnon, rc2.AllowAnon)
	assert.Equal(t, rc.TLS, rc2.TLS)
}

func TestRegistryConfig_JSONSerialization_AnonFalse(t *testing.T) {
	rc := RegistryConfig{
		Name:      "docker.io",
		Hostname:  "docker.io",
		AllowAnon: false,
		TLS:       true,
	}
	b, err := json.Marshal(rc)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"allowAnon":false`)
}

func TestGetRegistries_ReturnsConfigured(t *testing.T) {
	regs := []RegistryConfig{
		{Name: "zot.example.com", Hostname: "zot.example.com", AllowAnon: true, TLS: true},
		{Name: "registry.internal", URL: "registry.internal:5000", AllowAnon: true, TLS: false},
		{Name: "docker.io", Hostname: "docker.io", AllowAnon: false, TLS: true},
	}
	mag := Magazine{
		Config: MagConfig{
			Args:       map[string]string{},
			Registries: regs,
		},
	}
	got := mag.GetRegistries()
	assert.Len(t, got, 3)
	assert.Equal(t, "zot.example.com", got[0].EffectiveName())
	assert.Equal(t, "registry.internal", got[1].EffectiveName())
	assert.Equal(t, "registry.internal:5000", got[1].EffectiveURL())
	assert.Equal(t, "docker.io", got[2].EffectiveName())
}

func TestGetRegistries_EmptyReturnsNil(t *testing.T) {
	mag := Magazine{
		Config: MagConfig{
			Args: map[string]string{},
		},
	}
	got := mag.GetRegistries()
	assert.Empty(t, got)
}

// --- RegistryConfigFromHostname tests ---

func TestRegistryConfigFromHostname_Remote(t *testing.T) {
	rc := RegistryConfigFromHostname("zot.example.com")
	assert.Equal(t, "zot.example.com", rc.EffectiveName())
	assert.Equal(t, "zot.example.com", rc.EffectiveURL())
	assert.True(t, rc.AllowAnon)
	assert.True(t, rc.TLS)
}

func TestRegistryConfigFromHostname_Localhost(t *testing.T) {
	rc := RegistryConfigFromHostname("localhost:5000")
	assert.Equal(t, "localhost:5000", rc.EffectiveName())
	assert.True(t, rc.AllowAnon)
	assert.False(t, rc.TLS, "localhost should have TLS disabled")
}

func TestRegistryConfigFromHostname_PrivateNetwork(t *testing.T) {
	rc := RegistryConfigFromHostname("172.18.0.5:5000")
	assert.False(t, rc.TLS, "172.* should have TLS disabled")
}

func TestRegistryConfigFromHostname_TestRegistry(t *testing.T) {
	rc := RegistryConfigFromHostname("registry:5050")
	assert.False(t, rc.TLS, "registry:5050 should have TLS disabled")
}

func TestRegistryConfigFromHostname_DockerIO(t *testing.T) {
	rc := RegistryConfigFromHostname("docker.io")
	assert.True(t, rc.TLS, "docker.io should have TLS enabled")
	assert.True(t, rc.AllowAnon, "legacy string parsing defaults to AllowAnon true")
}

// --- EffectiveURL / EffectiveName tests ---

func TestEffectiveURL_FallsBackToName(t *testing.T) {
	rc := RegistryConfig{Name: "zot.example.com"}
	assert.Equal(t, "zot.example.com", rc.EffectiveURL())
}

func TestEffectiveURL_PrefersURL(t *testing.T) {
	rc := RegistryConfig{Name: "local-reg", URL: "registry:5050"}
	assert.Equal(t, "registry:5050", rc.EffectiveURL())
}

func TestEffectiveName_FallsBackToHostname(t *testing.T) {
	rc := RegistryConfig{Hostname: "zot.example.com"}
	assert.Equal(t, "zot.example.com", rc.EffectiveName())
}

func TestEffectiveName_PrefersName(t *testing.T) {
	rc := RegistryConfig{Name: "local-reg", Hostname: "old-name"}
	assert.Equal(t, "local-reg", rc.EffectiveName())
}

// --- normalize tests ---

func TestNormalize_FillsNameFromHostname(t *testing.T) {
	rc := RegistryConfig{Hostname: "zot.example.com"}
	rc.normalize()
	assert.Equal(t, "zot.example.com", rc.Name)
}

func TestNormalize_FillsHostnameFromName(t *testing.T) {
	rc := RegistryConfig{Name: "local-reg"}
	rc.normalize()
	assert.Equal(t, "local-reg", rc.Hostname)
}

// --- registryConfigFromMap / registryConfigFromStringMap tests ---

func TestRegistryConfigFromMap(t *testing.T) {
	m := map[interface{}]interface{}{
		"hostname":   "zot.example.com",
		"allow_anon": true,
		"tls":        true,
	}
	rc := registryConfigFromMap(m)
	assert.Equal(t, "zot.example.com", rc.EffectiveName())
	assert.Equal(t, "zot.example.com", rc.EffectiveURL())
	assert.True(t, rc.AllowAnon)
	assert.True(t, rc.TLS)
}

func TestRegistryConfigFromMap_WithNameAndURL(t *testing.T) {
	m := map[interface{}]interface{}{
		"name":       "local-reg",
		"url":        "registry:5050",
		"allow_anon": true,
		"tls":        false,
	}
	rc := registryConfigFromMap(m)
	assert.Equal(t, "local-reg", rc.EffectiveName())
	assert.Equal(t, "registry:5050", rc.EffectiveURL())
	assert.True(t, rc.AllowAnon)
	assert.False(t, rc.TLS)
}

func TestRegistryConfigFromMap_AnonFalse(t *testing.T) {
	m := map[interface{}]interface{}{
		"hostname":   "docker.io",
		"allow_anon": false,
		"tls":        true,
	}
	rc := registryConfigFromMap(m)
	assert.Equal(t, "docker.io", rc.EffectiveName())
	assert.False(t, rc.AllowAnon)
	assert.True(t, rc.TLS)
}

func TestRegistryConfigFromMap_Defaults(t *testing.T) {
	// Only hostname set — defaults should be TLS: true, AllowAnon: true
	m := map[interface{}]interface{}{
		"hostname": "registry.example.com",
	}
	rc := registryConfigFromMap(m)
	assert.Equal(t, "registry.example.com", rc.EffectiveName())
	assert.True(t, rc.AllowAnon)
	assert.True(t, rc.TLS)
}

func TestRegistryConfigFromStringMap(t *testing.T) {
	m := map[string]interface{}{
		"hostname":   "zot.example.com",
		"allow_anon": true,
		"tls":        false,
	}
	rc := registryConfigFromStringMap(m)
	assert.Equal(t, "zot.example.com", rc.EffectiveName())
	assert.True(t, rc.AllowAnon)
	assert.False(t, rc.TLS)
}

func TestRegistryConfigFromStringMap_WithNameAndURL(t *testing.T) {
	m := map[string]interface{}{
		"name":       "local-reg",
		"url":        "registry:5050",
		"allow_anon": true,
		"tls":        false,
	}
	rc := registryConfigFromStringMap(m)
	assert.Equal(t, "local-reg", rc.EffectiveName())
	assert.Equal(t, "registry:5050", rc.EffectiveURL())
}

// --- AllowsUnauthorizedPull tests ---

func TestAllowsUnauthorizedPull_ConfiguredAnonTrue(t *testing.T) {
	m := Magazine{
		Config: MagConfig{
			Args: map[string]string{},
			Registries: []RegistryConfig{
				{Name: "zot.example.com", Hostname: "zot.example.com", AllowAnon: true, TLS: true},
			},
		},
	}
	up := types.Upstream{Reg: "zot.example.com"}
	assert.True(t, m.AllowsUnauthorizedPull(up))
}

func TestAllowsUnauthorizedPull_MatchByURL(t *testing.T) {
	m := Magazine{
		Config: MagConfig{
			Args: map[string]string{},
			Registries: []RegistryConfig{
				{Name: "local-reg", URL: "registry:5050", AllowAnon: true, TLS: false},
			},
		},
	}
	// Upstream stores EffectiveURL as Reg
	up := types.Upstream{Reg: "registry:5050"}
	assert.True(t, m.AllowsUnauthorizedPull(up))
}

func TestAllowsUnauthorizedPull_ConfiguredAnonFalse(t *testing.T) {
	m := Magazine{
		Config: MagConfig{
			Args: map[string]string{},
			Registries: []RegistryConfig{
				{Name: "docker.io", Hostname: "docker.io", AllowAnon: false, TLS: true},
			},
		},
	}
	up := types.Upstream{Reg: "docker.io"}
	assert.False(t, m.AllowsUnauthorizedPull(up))
}

func TestAllowsUnauthorizedPull_UnknownDefaultsDenied(t *testing.T) {
	m := Magazine{
		Config: MagConfig{
			Args: map[string]string{},
			Registries: []RegistryConfig{
				{Name: "zot.example.com", Hostname: "zot.example.com", AllowAnon: true, TLS: true},
			},
		},
	}
	up := types.Upstream{Reg: "unknown.example.com"}
	assert.False(t, m.AllowsUnauthorizedPull(up))
}

func TestAllowsUnauthorizedPull_TestRegistryOverride(t *testing.T) {
	m := Magazine{
		Config: MagConfig{
			Args: map[string]string{"TEST_REGISTRY": "localhost:5050"},
			Registries: []RegistryConfig{
				{Name: "docker.io", Hostname: "docker.io", AllowAnon: false, TLS: true},
			},
		},
	}
	// TEST_REGISTRY override should always return true
	up := types.Upstream{Reg: "docker.io"}
	assert.True(t, m.AllowsUnauthorizedPull(up))
}

// --- ResolveUpstreamRegistry tests ---

func newMagWithRegistries(hostnames ...string) Magazine {
	var regs []RegistryConfig
	for _, h := range hostnames {
		regs = append(regs, RegistryConfig{Name: h, Hostname: h, AllowAnon: true, TLS: true})
	}
	return Magazine{
		Config: MagConfig{
			Args:       map[string]string{},
			Registries: regs,
		},
	}
}

func TestResolveUpstreamRegistry_PrefixMatch(t *testing.T) {
	m := newMagWithRegistries("zot.example.com", "docker.io")
	reg, stripped, found := m.ResolveUpstreamRegistry("zot.example.com/library/alpine")
	assert.True(t, found)
	assert.Equal(t, "zot.example.com", reg)
	assert.Equal(t, "library/alpine", stripped)
}

func TestResolveUpstreamRegistry_NoMatch(t *testing.T) {
	m := newMagWithRegistries("zot.example.com")
	reg, stripped, found := m.ResolveUpstreamRegistry("library/alpine")
	assert.False(t, found)
	assert.Equal(t, "", reg)
	assert.Equal(t, "library/alpine", stripped)
}

func TestResolveUpstreamRegistry_SingleSegment(t *testing.T) {
	m := newMagWithRegistries("zot.example.com")
	reg, stripped, found := m.ResolveUpstreamRegistry("alpine")
	assert.False(t, found)
	assert.Equal(t, "", reg)
	assert.Equal(t, "alpine", stripped)
}

func TestResolveUpstreamRegistry_PortBearingHostname(t *testing.T) {
	m := newMagWithRegistries("zot.example.com:5000")
	reg, stripped, found := m.ResolveUpstreamRegistry("zot.example.com:5000/library/alpine")
	assert.True(t, found)
	assert.Equal(t, "zot.example.com:5000", reg)
	assert.Equal(t, "library/alpine", stripped)
}

func TestResolveUpstreamRegistry_SecondRegistryMatch(t *testing.T) {
	m := newMagWithRegistries("zot.example.com", "docker.io")
	reg, stripped, found := m.ResolveUpstreamRegistry("docker.io/library/alpine")
	assert.True(t, found)
	assert.Equal(t, "docker.io", reg)
	assert.Equal(t, "library/alpine", stripped)
}

func TestResolveUpstreamRegistry_UnknownPrefix(t *testing.T) {
	m := newMagWithRegistries("zot.example.com")
	reg, stripped, found := m.ResolveUpstreamRegistry("myorg/myimage")
	assert.False(t, found)
	assert.Equal(t, "", reg)
	assert.Equal(t, "myorg/myimage", stripped)
}

// --- Name/URL split: resolve matches on Name (slug), returns URL ---

func TestResolveUpstreamRegistry_NameURLSplit(t *testing.T) {
	// The slug "local-reg" is used in the image path, but the URL
	// "registry:5050" is returned for upstream connections.
	m := Magazine{
		Config: MagConfig{
			Args: map[string]string{},
			Registries: []RegistryConfig{
				{Name: "local-reg", URL: "registry:5050", AllowAnon: true, TLS: false},
			},
		},
	}
	regURL, stripped, found := m.ResolveUpstreamRegistry("local-reg/os/alpine")
	assert.True(t, found, "slug 'local-reg' should match as a configured registry prefix")
	assert.Equal(t, "registry:5050", regURL, "returned URL should be the EffectiveURL, not the slug")
	assert.Equal(t, "os/alpine", stripped)
}

func TestResolveUpstreamRegistry_LegacyHostnameStillWorks(t *testing.T) {
	// Legacy config with only hostname (no name, no url) — should still work.
	m := Magazine{
		Config: MagConfig{
			Args: map[string]string{},
			Registries: []RegistryConfig{
				{Hostname: "zot.example.com", AllowAnon: true, TLS: true},
			},
		},
	}
	reg, stripped, found := m.ResolveUpstreamRegistry("zot.example.com/library/alpine")
	assert.True(t, found)
	assert.Equal(t, "zot.example.com", reg)
	assert.Equal(t, "library/alpine", stripped)
}

// --- DiscoverUpstream targeting tests (unit-level, no real registry) ---
// These test the routing logic — which registries get tried based on targetReg.
// Full integration tests with a real registry are in collection_test.go.

func TestDiscoverUpstream_UnknownTargetReturnsError(t *testing.T) {
	m := newMagWithRegistries("zot.example.com")
	_, _, _, err := m.DiscoverUpstream(context.Background(), "library/alpine", "3.23", "unknown.example.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestDiscoverUpstream_NoRegistriesConfiguredFallsBackToArgs(t *testing.T) {
	// When MagConfig.Registries is empty but Args["registries"] is set,
	// DiscoverUpstream falls back to the flat string. This verifies backward compat.
	m := Magazine{
		Config: MagConfig{
			Args:       map[string]string{"registries": "zot.example.com"},
			Registries: nil,
		},
	}
	// This will fail at the HEAD call (no real registry), but should NOT error on "not configured"
	_, _, found, err := m.DiscoverUpstream(context.Background(), "library/alpine", "3.23")
	// err may be non-nil (connection refused) but it shouldn't be "Failed to get 'registries'"
	if err != nil {
		assert.NotContains(t, err.Error(), "Failed to get 'registries'")
	}
	_ = found
}

// --- ValidateRegistries tests ---

func TestValidateRegistries_RejectsEmptyName(t *testing.T) {
	regs := []RegistryConfig{
		{Name: "zot.example.com", AllowAnon: true, TLS: true},
		{AllowAnon: true, TLS: false}, // empty name AND hostname
	}
	errs := ValidateRegistries(regs)
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "empty name")
}

func TestValidateRegistries_AcceptsValidEntries(t *testing.T) {
	regs := []RegistryConfig{
		{Name: "zot.example.com", AllowAnon: true, TLS: true},
		{Name: "local-reg", URL: "registry:5050", AllowAnon: true, TLS: false},
	}
	errs := ValidateRegistries(regs)
	assert.Empty(t, errs)
}

func TestValidateRegistries_AcceptsLegacyHostnameOnly(t *testing.T) {
	regs := []RegistryConfig{
		{Hostname: "zot.example.com", AllowAnon: true, TLS: true},
	}
	errs := ValidateRegistries(regs)
	assert.Empty(t, errs, "legacy hostname-only entries should be valid")
}

func TestValidateRegistries_EmptySliceIsValid(t *testing.T) {
	errs := ValidateRegistries(nil)
	assert.Empty(t, errs)
}

// --- GetBlobRedirectURL tests ---

func TestGetBlobRedirectURL_TLSFalse_HTTP(t *testing.T) {
	m := Magazine{
		Config: MagConfig{
			Args: map[string]string{},
			Registries: []RegistryConfig{
				{Name: "local-reg", URL: "registry:5050", AllowAnon: true, TLS: false},
			},
		},
	}
	up := types.Upstream{Reg: "registry:5050", Repo: "os/alpine"}
	url := m.GetBlobRedirectURL(up, "sha256:abc123")
	assert.Equal(t, "http://registry:5050/v2/os/alpine/blobs/sha256:abc123", url)
}

func TestGetBlobRedirectURL_TLSTrue_HTTPS(t *testing.T) {
	m := Magazine{
		Config: MagConfig{
			Args: map[string]string{},
			Registries: []RegistryConfig{
				{Name: "corp-reg", URL: "reg.corp:443", AllowAnon: true, TLS: true},
			},
		},
	}
	up := types.Upstream{Reg: "reg.corp:443", Repo: "library/alpine"}
	url := m.GetBlobRedirectURL(up, "sha256:def456")
	assert.Equal(t, "https://reg.corp:443/v2/library/alpine/blobs/sha256:def456", url)
}

func TestGetBlobRedirectURL_LegacyHostname_TLSFalse(t *testing.T) {
	m := Magazine{
		Config: MagConfig{
			Args: map[string]string{},
			Registries: []RegistryConfig{
				{Hostname: "zot.example.com", AllowAnon: true, TLS: false},
			},
		},
	}
	up := types.Upstream{Reg: "zot.example.com", Repo: "library/alpine"}
	url := m.GetBlobRedirectURL(up, "sha256:aaa")
	assert.True(t, strings.HasPrefix(url, "http://"), "TLS=false legacy hostname should produce http://")
}

func TestGetBlobRedirectURL_LegacyHostname_TLSTrue(t *testing.T) {
	m := Magazine{
		Config: MagConfig{
			Args: map[string]string{},
			Registries: []RegistryConfig{
				{Hostname: "docker.io", AllowAnon: false, TLS: true},
			},
		},
	}
	up := types.Upstream{Reg: "docker.io", Repo: "library/alpine"}
	url := m.GetBlobRedirectURL(up, "sha256:bbb")
	assert.True(t, strings.HasPrefix(url, "https://"), "TLS=true should produce https://")
}

func TestGetBlobRedirectURL_UnknownReg_DefaultsHTTPS(t *testing.T) {
	m := Magazine{
		Config: MagConfig{
			Args: map[string]string{},
			Registries: []RegistryConfig{
				{Name: "zot.example.com", Hostname: "zot.example.com", AllowAnon: true, TLS: true},
			},
		},
	}
	up := types.Upstream{Reg: "unknown.example.com", Repo: "myrepo"}
	url := m.GetBlobRedirectURL(up, "sha256:ccc")
	assert.True(t, strings.HasPrefix(url, "https://"), "unknown registries should default to https://")
}

func TestGetBlobRedirectURL_MatchOnName(t *testing.T) {
	// Legacy data where Upstream.Reg stores the slug name, not the URL
	m := Magazine{
		Config: MagConfig{
			Args: map[string]string{},
			Registries: []RegistryConfig{
				{Name: "local-reg", URL: "registry:5050", AllowAnon: true, TLS: false},
			},
		},
	}
	up := types.Upstream{Reg: "local-reg", Repo: "os/alpine"}
	url := m.GetBlobRedirectURL(up, "sha256:ddd")
	assert.True(t, strings.HasPrefix(url, "http://"), "match on EffectiveName should also pick up TLS=false")
}

func TestGetBlobRedirectURL_DigestFallback(t *testing.T) {
	m := Magazine{
		Config: MagConfig{
			Args:       map[string]string{},
			Registries: []RegistryConfig{{Name: "r", Hostname: "r", TLS: true}},
		},
	}
	up := types.Upstream{Reg: "r", Repo: "img", Digest: "sha256:fallback"}
	url := m.GetBlobRedirectURL(up, "")
	assert.Contains(t, url, "sha256:fallback", "empty digest arg should fall back to up.Digest")
}

func TestGetBlobRedirectURL_DigestOverride(t *testing.T) {
	m := Magazine{
		Config: MagConfig{
			Args:       map[string]string{},
			Registries: []RegistryConfig{{Name: "r", Hostname: "r", TLS: true}},
		},
	}
	up := types.Upstream{Reg: "r", Repo: "img", Digest: "sha256:original"}
	url := m.GetBlobRedirectURL(up, "sha256:override")
	assert.Contains(t, url, "sha256:override", "non-empty digest arg should override up.Digest")
	assert.NotContains(t, url, "sha256:original")
}

func TestGetBlobRedirectURL_RepoWithSlashes(t *testing.T) {
	m := Magazine{
		Config: MagConfig{
			Args: map[string]string{},
			Registries: []RegistryConfig{
				{Name: "local-reg", URL: "registry:5050", TLS: false},
			},
		},
	}
	up := types.Upstream{Reg: "registry:5050", Repo: "os/alpine"}
	url := m.GetBlobRedirectURL(up, "sha256:eee")
	assert.Equal(t, "http://registry:5050/v2/os/alpine/blobs/sha256:eee", url)
}

// --- inferTLS tests ---

func TestInferTLS(t *testing.T) {
	tests := []struct {
		hostname string
		expected bool
	}{
		{"zot.example.com", true},
		{"docker.io", true},
		{"ghcr.io", true},
		{"localhost:5000", false},
		{"localhost", false},
		{"172.18.0.5:5000", false},
		{"172.0.0.1", false},
		{"registry:5050", false},
		{"registry.internal:5000", true},
	}
	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			assert.Equal(t, tt.expected, inferTLS(tt.hostname), "inferTLS(%q)", tt.hostname)
		})
	}
}
