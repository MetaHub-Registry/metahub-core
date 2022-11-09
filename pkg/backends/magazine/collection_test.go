package magazine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_Collection(t *testing.T) {
	bs := utils.NewBootstrap()
	_, err := SetupTest(bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	// Test
}
