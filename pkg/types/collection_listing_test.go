package types_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func Test_NewCollectionListing(t *testing.T) {
	cl := types.NewCollectionListing("repo", "tag")
	assert.Equal(t, "repo", cl.Repo)
	assert.Equal(t, "tag", cl.Tag)
	assert.Equal(t, int64(-1), cl.ID)
	assert.Equal(t, int64(0), cl.ManifestCount)
	assert.Equal(t, 0, len(cl.Manifests))
}
