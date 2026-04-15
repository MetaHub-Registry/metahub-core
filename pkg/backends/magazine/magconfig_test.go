package magazine

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func TestRegistryConfig_JSONSerialization(t *testing.T) {
	rc := RegistryConfig{
		Hostname:  "zot.example.com",
		AllowAnon: true,
		TLS:       true,
	}
	b, err := json.Marshal(rc)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"hostname":"zot.example.com"`)
	assert.Contains(t, string(b), `"allowAnon":true`)
	assert.Contains(t, string(b), `"tls":true`)

	var rc2 RegistryConfig
	err = json.Unmarshal(b, &rc2)
	require.NoError(t, err)
	assert.Equal(t, rc, rc2)
}

func TestRegistryConfig_JSONSerialization_AnonFalse(t *testing.T) {
	rc := RegistryConfig{
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
		{Hostname: "zot.example.com", AllowAnon: true, TLS: true},
		{Hostname: "registry.internal:5000", AllowAnon: true, TLS: false},
		{Hostname: "docker.io", AllowAnon: false, TLS: true},
	}
	mag := Magazine{
		Config: MagConfig{
			Args:       map[string]string{},
			Registries: regs,
		},
	}
	got := mag.GetRegistries()
	assert.Len(t, got, 3)
	assert.Equal(t, "zot.example.com", got[0].Hostname)
	assert.Equal(t, "registry.internal:5000", got[1].Hostname)
	assert.Equal(t, "docker.io", got[2].Hostname)
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
	assert.Equal(t, "zot.example.com", rc.Hostname)
	assert.True(t, rc.AllowAnon)
	assert.True(t, rc.TLS)
}

func TestRegistryConfigFromHostname_Localhost(t *testing.T) {
	rc := RegistryConfigFromHostname("localhost:5000")
	assert.Equal(t, "localhost:5000", rc.Hostname)
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

// --- registryConfigFromMap / registryConfigFromStringMap tests ---

func TestRegistryConfigFromMap(t *testing.T) {
	m := map[interface{}]interface{}{
		"hostname":   "zot.example.com",
		"allow_anon": true,
		"tls":        true,
	}
	rc := registryConfigFromMap(m)
	assert.Equal(t, "zot.example.com", rc.Hostname)
	assert.True(t, rc.AllowAnon)
	assert.True(t, rc.TLS)
}

func TestRegistryConfigFromMap_AnonFalse(t *testing.T) {
	m := map[interface{}]interface{}{
		"hostname":   "docker.io",
		"allow_anon": false,
		"tls":        true,
	}
	rc := registryConfigFromMap(m)
	assert.Equal(t, "docker.io", rc.Hostname)
	assert.False(t, rc.AllowAnon)
	assert.True(t, rc.TLS)
}

func TestRegistryConfigFromMap_Defaults(t *testing.T) {
	// Only hostname set — defaults should be TLS: true, AllowAnon: true
	m := map[interface{}]interface{}{
		"hostname": "registry.example.com",
	}
	rc := registryConfigFromMap(m)
	assert.Equal(t, "registry.example.com", rc.Hostname)
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
	assert.Equal(t, "zot.example.com", rc.Hostname)
	assert.True(t, rc.AllowAnon)
	assert.False(t, rc.TLS)
}

// --- AllowsUnauthorizedPull tests ---

func TestAllowsUnauthorizedPull_ConfiguredAnonTrue(t *testing.T) {
	m := Magazine{
		Config: MagConfig{
			Args: map[string]string{},
			Registries: []RegistryConfig{
				{Hostname: "zot.example.com", AllowAnon: true, TLS: true},
			},
		},
	}
	up := types.Upstream{Reg: "zot.example.com"}
	assert.True(t, m.AllowsUnauthorizedPull(up))
}

func TestAllowsUnauthorizedPull_ConfiguredAnonFalse(t *testing.T) {
	m := Magazine{
		Config: MagConfig{
			Args: map[string]string{},
			Registries: []RegistryConfig{
				{Hostname: "docker.io", AllowAnon: false, TLS: true},
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
				{Hostname: "zot.example.com", AllowAnon: true, TLS: true},
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
				{Hostname: "docker.io", AllowAnon: false, TLS: true},
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
		regs = append(regs, RegistryConfig{Hostname: h, AllowAnon: true, TLS: true})
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
