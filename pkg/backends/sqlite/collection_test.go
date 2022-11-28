package sqlite

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_Collection_Insert(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetSqlEmptyAll()
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer TeardownTest(bs)
	err = sq.Collection.CreateViews()
	assert.NoError(t, err)
	c := types.Collection{
		Repo: "qnib/test",
		Tag:  types.NewCollectionTag("1.0"),
	}
	err = sq.Collection.Insert(&c)
	assert.NoError(t, err)
	assert.NotEqual(t, int64(-1), c.SqlID)
	cols, err := sq.Collection.GetAll()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(cols))
}

func Test_Collection_GetInsert(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetSqlEmptyAll()
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer TeardownTest(bs)
	err = sq.Collection.CreateViews()
	assert.NoError(t, err)
	col, err := sq.GetInsertCollectionByRepoTag("qnib/test", "1.0")
	assert.NoError(t, err)
	assert.NotEqual(t, int64(-1), col.SqlID)
	assert.Equal(t, "qnib/test", col.Repo)
	assert.Equal(t, "1.0", col.Tag.Tag)
	cols, err := sq.Collection.GetAll()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(cols))
}

func Test_Collection_Get(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetSqlEmptyAll()
	bs.AddBootstrap("users")
	bs.AddBootstrap("profiles")
	bs.AddBootstrap("platforms")
	bs.AddBootstrap("upstreams:qnib/featuretest:arm64")
	bs.AddBootstrap("layers:qnib/featuretest:arm64")
	bs.AddBootstrap("config:qnib/featuretest:arm64")
	bs.AddBootstrap("manifest:qnib/featuretest:arm64")
	bs.AddBootstrap("manifestlist:qnib/featuretest:arm64")
	bs.AddBootstrap("collection:qnib/featuretest:arm64")
	//bs.SetSqlite("Desktop/metahub.db")
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer TeardownTest(bs)
	// Tests
	cols, err := sq.Collection.GetAll()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(cols))
	col := cols[0]
	logrus.Warnf("col.ID: %#v", col.ID)
	gotCol, err := sq.Collection.GetCollectionByID(col.ID)
	assert.NoError(t, err)
	assert.Equal(t, "qnib/featuretest", gotCol.Repo)
	gotCols1, err := sq.Collection.GetCollectionsByRepo("")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(gotCols1))
	gotCols2, err := sq.Collection.GetCollectionsByRepo("qnib")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(gotCols2))
	gotCols3, err := sq.Collection.GetCollectionsByRepo("qnib/featuretest")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(gotCols3))
	gotCols4, err := sq.Collection.GetCollectionsByRepoLike("qnib/featuretest")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(gotCols4))
	gotCol1, found, err := sq.Collection.GetCollectionByRepoTag("qnib/featuretest", "arm64")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "qnib/featuretest", gotCol1.Repo)
	gotTags, err := sq.Collection.GetTagsListByRepo("qnib/featuretest")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(gotTags.Tags))
}

func Test_Collection_ScrubCollectionsByManifestListID(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetSqlEmptyAll()
	bs.AddBootstrap("users")
	bs.AddBootstrap("profiles")
	bs.AddBootstrap("platforms")
	bs.AddBootstrap("upstreams:qnib/featuretest:arm64")
	bs.AddBootstrap("layers:qnib/featuretest:arm64")
	bs.AddBootstrap("config:qnib/featuretest:arm64")
	bs.AddBootstrap("manifest:qnib/featuretest:arm64")
	bs.AddBootstrap("manifest:qnib/featuretest:amd64")
	bs.AddBootstrap("manifestlist:qnib/featuretest:arm64")
	//bs.AddBootstrap("manifestlist:qnib/featuretest:amd64")
	bs.AddBootstrap("collection:qnib/featuretest:arm64")
	//bs.AddBootstrap("collection:qnib/featuretest:amd64")
	//bs.SetSqlite("Desktop/metahub.db")
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer TeardownTest(bs)
	// Tests
	mlId, err := br.GetInt64("manifestlist:qnib/featuretest:arm64#id")
	_ = mlId
	assert.NoError(t, err)
	err = sq.Collection.ScrubCollectionsByManifestListID(mlId)
	assert.NoError(t, err)
}
