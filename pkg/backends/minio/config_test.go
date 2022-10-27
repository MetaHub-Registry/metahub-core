package minio

import (
	"context"
	"testing"

	"github.com/containers/image/v5/types"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
)

func Test_ConfigGetter(t *testing.T) {
	bi := types.BlobInfo{
		Digest: digest.Digest(featureTestGenericCfgDigest),
	}
	cfg, err := min.configGetter(bi)
	assert.NoError(t, err)
	assert.Equal(t, featureTestGenericCfg, cfg)
}

func Test_GetConfig(t *testing.T) {
	ctx := context.Background()
	err := min.PutConfig(ctx, []byte(featureTestGenericCfg), featureTestGenericCfgDigest)
	assert.NoError(t, err)
	up := upstream.UpstreamTriple{
		Digest: digest.Digest(featureTestGenericCfgDigest),
	}
	cfg, err := min.ConfigGetObj(ctx, up.Digest.String())
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	_, found, err := min.GetConfig(ctx, up)
	assert.NoError(t, err)
	assert.True(t, found)
}

func Test_ConfigRemoveObj(t *testing.T) {
	ctx := context.Background()
	err := min.PutConfig(ctx, []byte(featureTestGenericCfg), "sha256:1234567890")
	assert.NoError(t, err)
	err = min.ConfigRemoveObj(ctx, "sha256:1234567890")
	assert.NoError(t, err)
}
