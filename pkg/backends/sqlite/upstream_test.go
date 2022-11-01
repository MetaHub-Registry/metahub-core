package sqlite

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_Upstream_Insert(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetSqlEmptyAll()
	//bs.SetSqlite("Desktop/metahub.db")
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer TeardownTest(bs)
	// Test
	up1 := types.Upstream{
		Reg:  "metahub-registry.org",
		Repo: "qnib/test",
		Tag:  "1.0",
	}
	found, err := sq.Upstream.GetByRequest(&up1)
	assert.NoError(t, err)
	assert.False(t, found)

	up2 := types.Upstream{
		Reg:  "metahub-registry.org",
		Repo: "qnib/test",
		Tag:  "1.0",
	}
	err = sq.Upstream.Insert(&up1)
	assert.NoError(t, err)
	assert.NotEqual(t, int64(-1), up1.SqlID)
	found, err = sq.Upstream.GetByRequest(&up2)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, up1.SqlID, up2.SqlID)
}

func Test_Upstream_GetCreateByRequest(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetSqlEmptyAll()
	//bs.SetSqlite("Desktop/metahub.db")
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer TeardownTest(bs)
	// Test
	req := types.Request{
		Profile: "generic",
		Repo:    "qnib/test",
		Tag:     "1.0",
		//Digest:  "sha256:1234567890abcdef",
	}
	up, err := sq.Upstream.GetCreateByRequest(req)
	assert.NoError(t, err)
	assert.NotEqual(t, int64(-1), up.SqlID)
}
