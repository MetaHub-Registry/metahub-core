package sqlite

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_CollectionTag_CRUD(t *testing.T) {
	bs := utils.NewBootstrap()
	//bs.SetSqlite("Desktop/metahub.db")
	br, err := SetupTest(bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	_ = br
	// Tests
	colTag := types.CollectionTag_arm()
	err = sq.CollectionTag.Insert(&colTag)
	assert.NoError(t, err)
	assert.NotEqual(t, -1, colTag.ID)
	gotColTag, found, err := sq.CollectionTag.GetByTag("arm64")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "arm64", gotColTag.Tag)
	assert.Equal(t, colTag.ID, gotColTag.ID)
	err = sq.CollectionTag.RemoveByID(gotColTag.ID)
	assert.NoError(t, err)
	_, found, err = sq.CollectionTag.GetByTag("arm64")
	assert.NoError(t, err)
	assert.False(t, found)
	gotTag, err := sq.CollectionTag.GetCreateByRequest(types.Request{Tag: "arm64"})
	assert.NoError(t, err)
	assert.Equal(t, "arm64", gotTag.Tag)
	assert.NotEqual(t, -1, gotTag.ID)
	gotTag2, err := sq.CollectionTag.GetCreateByRequest(types.Request{Tag: "arm64"})
	assert.NoError(t, err)
	assert.Equal(t, "arm64", gotTag2.Tag)
	assert.Equal(t, gotTag.ID, gotTag2.ID)
}
