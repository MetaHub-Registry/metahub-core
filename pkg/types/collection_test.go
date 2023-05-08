package types_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func TestNewCollection(t *testing.T) {
	c := types.NewCollection("repo", "tag")
	assert.Equal(t, int64(-1), c.ID)
	assert.Equal(t, "repo", c.Repo)
	assert.Equal(t, "tag", c.Tag)
}
