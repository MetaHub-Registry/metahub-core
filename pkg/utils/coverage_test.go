package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandMediaType(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"oci_image_manifest_v1JSON", "application/vnd.oci.image.manifest.v1+json"},
		{"docker_distribution_manifest_v2JSON", "application/vnd.docker.distribution.manifest.v2+json"},
		{"application/vnd.oci.image.index.v1+json", "application/vnd.oci.image.index.v1+json"},
		{"oci_image_layer_v1_tar:GZIP", "application/vnd.oci.image.layer.v1.tar++gzip"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got := ExpandMediaType(tc.in)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestJsonPrettyPrint(t *testing.T) {
	in := []byte(`{"key":"value"}`)
	out := JsonPrettyPrint(in)
	assert.Contains(t, out, "\"key\"")
	assert.Contains(t, out, "\t")

	// Invalid JSON returns input as-is
	invalid := []byte(`{invalid`)
	out2 := JsonPrettyPrint(invalid)
	assert.Equal(t, string(invalid), out2)
}

func TestCreateLabelFromTypeAndDigest_VariousMediaTypes(t *testing.T) {
	// Just verify it returns a non-empty label for various inputs
	tests := []string{
		"application/vnd.oci.image.index.v1+json",
		"application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.oci.image.manifest.v1+json",
		"custom/type",
	}
	for _, mt := range tests {
		t.Run(mt, func(t *testing.T) {
			label := CreateLabelFromTypeAndDigest(mt, "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")
			assert.NotEmpty(t, label)
		})
	}
}

func TestIsReverseDNS(t *testing.T) {
	assert.True(t, IsReverseDNS("org.opencontainers.image.title"))
	assert.True(t, IsReverseDNS("com.docker.compose.service"))
	assert.False(t, IsReverseDNS("simple"))
	assert.False(t, IsReverseDNS("two.parts"))
	assert.False(t, IsReverseDNS(""))
}

func TestPrettyString(t *testing.T) {
	s, err := PrettyString(`{"a":"b"}`)
	require.NoError(t, err)
	assert.Contains(t, s, "\"a\"")
}

func TestPrettyString_InvalidJSON(t *testing.T) {
	_, err := PrettyString(`{invalid}`)
	assert.Error(t, err)
}

func TestPrintJSON(t *testing.T) {
	// PrintJSON prints to stdout; just verify no error for valid input
	err := PrintJSON(`{"a":"b"}`)
	assert.NoError(t, err)
}

func TestBootstrapNewAndMethods(t *testing.T) {
	bs := NewBootstrap()
	bs.SetEmptyAll()
	assert.True(t, bs.EmptyAll)

	bs.AddBootstrap("users")
	bs.AddBootstrap("platforms:generic")
	assert.Len(t, bs.Bootstraps, 2)

	bs.SetYAMLPrefix("/some/path")
	assert.Equal(t, "/some/path", bs.YAMLPrefix)
}

func TestBootstrapResult(t *testing.T) {
	br := NewBootstrapResult()
	br.Add("key1", "val1")
	v, err := br.Get("key1")
	require.NoError(t, err)
	assert.Equal(t, "val1", v)

	_, err = br.Get("nonexistent")
	assert.Error(t, err)

	br.Add("num", "42")
	n, err := br.GetInt("num")
	require.NoError(t, err)
	assert.Equal(t, 42, n)

	n64, err := br.GetInt64("num")
	require.NoError(t, err)
	assert.Equal(t, int64(42), n64)

	_, err = br.GetInt("nonexistent")
	assert.Error(t, err)

	_, err = br.GetInt64("nonexistent")
	assert.Error(t, err)
}

func TestBootstrapResultToLines(t *testing.T) {
	br := NewBootstrapResult()
	br.Add("a", "1")
	br.Add("b", "2")
	lines := br.ToLines()
	assert.GreaterOrEqual(t, len(lines), 2)
}

func TestEnvHelperToSlice(t *testing.T) {
	e := NewEnvStore()
	_ = e.AddEnv("KEY1=val1")
	_ = e.AddEnv("KEY2=val2")
	s := e.ToSlice()
	assert.Len(t, s, 2)
	assert.Contains(t, s, "KEY1=val1")
	assert.Contains(t, s, "KEY2=val2")
}

func TestBase64Decode(t *testing.T) {
	// Base64Decode returns the decoded string (empty on error)
	decoded := Base64Decode("dGVzdA==") // "test" in base64
	assert.Equal(t, "test", decoded)
}
