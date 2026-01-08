package types

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewConfigFromBytes(t *testing.T) {
	ctx := context.Background()
	cfg, err := NewConfigFromBytes(ctx, ConfigByteFtArm64())
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "arm64", cfg.Cfg.Architecture)
	if len(cfg.Cfg.Config.Env) != 3 {
		t.Logf("Env: %#v", cfg.Cfg.Config.Env)
		t.Fatal()
	}
	if len(cfg.Cfg.Config.Labels) != 1 {
		t.Logf("Labels: %#v", cfg.Cfg.Config.Labels)
		t.Fatal()
	}
	_, err = NewConfigFromBytes(ctx, []byte(`{`))
	assert.Error(t, err)
	_, err = NewConfigFromBytes(ctx, ManifestReferenceByte_FT_arm())
	assert.Error(t, err)
}

func TestConfigToDescriptor(t *testing.T) {
	ctx := context.Background()
	cfg, err := NewConfigFromBytes(ctx, ConfigByteFtArm64())
	if err != nil {
		t.Fatal(err)
	}
	desc := cfg.ToDescriptor()
	assert.Equal(t, "application/vnd.docker.container.image.v1+json", desc.MediaType)
	assert.Equal(t, "sha256:cf5c26263929e84d7373d23b2692222cceceb3175b3325b183ead2ab6b5a5ce3", desc.Digest.String())
	assert.Equal(t, int64(2191), desc.Size)
}

func TestConfigWriteWWWHeader(t *testing.T) {
	ctx := context.Background()
	cfg, err := NewConfigFromBytes(ctx, ConfigByteFtArm64())
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	cfg.WriteWWW(rr)
	assert.Equal(t, "application/vnd.docker.container.image.v1+json", rr.Header().Get("Content-Type"))
	assert.Equal(t, "2191", rr.Header().Get("Content-Length"))
	assert.Equal(t, "sha256:cf5c26263929e84d7373d23b2692222cceceb3175b3325b183ead2ab6b5a5ce3", rr.Header().Get("Docker-Content-Digest"))
	assert.Equal(t, 2191, rr.Body.Len())
}

func TestToOCISpecV1Image(t *testing.T) {
	ctx := context.Background()
	cfg0, err := NewConfigFromBytes(ctx, ConfigByteFtArm64())
	if err != nil {
		t.Fatal(err)
	}
	v1 := cfg0.ToOCISpecV1Image()
	assert.Equal(t, "arm64", v1.Architecture)
	assert.Equal(t, "linux", v1.OS)
	assert.Equal(t, 0, len(v1.Config.Entrypoint))
	assert.Equal(t, 1, len(v1.Config.Cmd))

}

func TestConfigSwitchMediaTypes(t *testing.T) {
	ctx := context.Background()

	cfg0, err := NewConfigFromBytes(ctx, ConfigByteFtArm64())
	if err != nil {
		t.Fatal(err)
	}
	// Fail
	buFail1 := &OCIBlobUpload{
		MediaType: "FAIL",
	}
	err = cfg0.SwitchMediaType(buFail1, []byte{})
	if err == nil {
		t.Fatal("expected error on unknown media type")
	}
	buFail1 = &OCIBlobUpload{
		MediaType: "application/vnd.oci.image.config.v1+json",
	}
	err = cfg0.SwitchMediaType(buFail1, []byte{})
	if err == nil {
		t.Fatal("expected error missmatch media type")
	}
	// Docker
	bu := &OCIBlobUpload{
		Digest:    cfg0.Digest,
		Data:      cfg0.Data,
		MediaType: "application/vnd.docker.container.image.v1+json",
	}
	err = cfg0.SwitchMediaType(bu, cfg0.Data)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "application/vnd.docker.container.image.v1+json", cfg0.MediaType)
	err = cfg0.SwitchMediaType(bu, []byte(`{invalid json}`))
	if err == nil {
		t.Fatal("expected error on invalid json")
	}
	// Empty
	bu2 := &OCIBlobUpload{
		Digest:    cfg0.Digest,
		Data:      cfg0.Data,
		MediaType: "",
	}
	err = cfg0.SwitchMediaType(bu2, cfg0.Data)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "application/vnd.oci.image.config.v1+json", cfg0.MediaType)
	//
	bu3 := &OCIBlobUpload{
		Digest:    cfg0.Digest,
		Data:      cfg0.Data,
		MediaType: "application/vnd.oci.image.config.v1+json",
	}
	cfg0.MediaType = "application/vnd.oci.image.config.v1+json"
	err = cfg0.SwitchMediaType(bu3, cfg0.Data)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "application/vnd.oci.image.config.v1+json", cfg0.MediaType)
	err = cfg0.SwitchMediaType(bu, []byte(`{invalid json}`))
	if err == nil {
		t.Fatal("expected error on invalid json")
	}

}
