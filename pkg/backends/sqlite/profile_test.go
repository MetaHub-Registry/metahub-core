package sqlite

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_Profiles_Insert(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetSqlEmptyAll()
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer TeardownTest(bs)
	// Test
	p := types.Profile{
		Name: `__[ ]__|"?___'-@------@- '`,
	}
	err = sq.Profile.Insert(&p)
	assert.NoError(t, err)

}
