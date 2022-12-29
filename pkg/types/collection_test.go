package types_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func Test_NewCollection(t *testing.T) {
	c := types.NewCollection()
	assert.Equal(t, int64(-1), c.ID)
}
