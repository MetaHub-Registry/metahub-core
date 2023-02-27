package types

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewConfigFromBytes(t *testing.T) {
	cfg, err := NewConfigFromBytes(FT_arm_ConfigByte())
	assert.NoError(t, err)
	assert.Equal(t, "arm64", cfg.Cfg.Architecture)
	if len(cfg.Cfg.Config.Env) != 3 {
		t.Logf("Env: %#v", cfg.Cfg.Config.Env)
		t.Fatal()
	}
	if len(cfg.Cfg.Config.Labels) != 1 {
		t.Logf("Labels: %#v", cfg.Cfg.Config.Labels)
		t.Fatal()
	}
	_, err = NewConfigFromBytes([]byte(`{`))
	assert.Error(t, err)
	_, err = NewConfigFromBytes(ManifestReferenceByte_FT_arm())
	assert.Error(t, err)
}

func Test_Config_ToDescriptor(t *testing.T) {
	cfg, err := NewConfigFromBytes(FT_arm_ConfigByte())
	assert.NoError(t, err)
	desc := cfg.ToDescriptor()
	assert.Equal(t, "application/vnd.docker.container.image.v1+json", desc.MediaType)
	assert.Equal(t, "sha256:cf5c26263929e84d7373d23b2692222cceceb3175b3325b183ead2ab6b5a5ce3", desc.Digest.String())
	assert.Equal(t, int64(2191), desc.Size)
}

func Test_Config_WriteWWWHeader(t *testing.T) {
	cfg, err := NewConfigFromBytes(FT_arm_ConfigByte())
	assert.NoError(t, err)
	rr := httptest.NewRecorder()
	cfg.WriteWWW(rr)
	assert.Equal(t, "application/vnd.docker.container.image.v1+json", rr.Header().Get("Content-Type"))
	assert.Equal(t, "2191", rr.Header().Get("Content-Length"))
	assert.Equal(t, "sha256:cf5c26263929e84d7373d23b2692222cceceb3175b3325b183ead2ab6b5a5ce3", rr.Header().Get("Docker-Content-Digest"))
	assert.Equal(t, 2191, rr.Body.Len())
}

func Test_ToOCISpecV1Image(t *testing.T) {
	cfg0, err := NewConfigFromBytes(FT_arm_ConfigByte())
	assert.NoError(t, err)
	v1 := cfg0.ToOCISpecV1Image()
	assert.Equal(t, "arm64", v1.Architecture)
	assert.Equal(t, "linux", v1.OS)
	assert.Equal(t, 0, len(v1.Config.Entrypoint))
	assert.Equal(t, 1, len(v1.Config.Cmd))

}
