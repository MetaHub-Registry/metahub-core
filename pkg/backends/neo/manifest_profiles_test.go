package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestManifestGetProfileLabels_EmptyDigests(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	result, err := db.ManifestGetProfileLabels(ctx, []string{})
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestManifestGetProfileLabels_UnknownDigests(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()

	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	result, err := db.ManifestGetProfileLabels(ctx, []string{"sha256:deadbeef"})
	assert.NoError(t, err)
	assert.Empty(t, result)
}
