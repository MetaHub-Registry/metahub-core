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
	cfg.WriteWWWHeader(rr)
	assert.Equal(t, "application/vnd.docker.container.image.v1+json", rr.Header().Get("Content-Type"))
	assert.Equal(t, "2191", rr.Header().Get("Content-Length"))
	assert.Equal(t, "sha256:cf5c26263929e84d7373d23b2692222cceceb3175b3325b183ead2ab6b5a5ce3", rr.Header().Get("Docker-Content-Digest"))

	cfg.WriteWWWBody(rr)
	assert.Equal(t, 2191, rr.Body.Len())

}
