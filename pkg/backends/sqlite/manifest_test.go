package sqlite

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_Manifest_Insert(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetSqlEmptyAll()
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer TeardownTest(bs)
	// Test
	man1 := types.FT_arm_Manifest()
	err = sq.Manifest.Insert(&man1)
	assert.NoError(t, err)
	assert.NotEqual(t, int64(-1), man1.SqlID)
}
func Test_Manifest_GetInsert(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetSqlEmptyAll()
	//bs.SetSqlite("Desktop/metahub.db")
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer TeardownTest(bs)
	// Test
	man1 := types.FT_arm_Manifest()
	err = sq.Manifest.GetInsert(&man1)
	assert.NoError(t, err)
	assert.NotEqual(t, int64(-1), man1.SqlID)
	err = sq.Manifest.GetInsert(&man1)
	assert.NoError(t, err)
	assert.NotEqual(t, int64(-1), man1.SqlID)
	_, found, err := sq.Manifest.GetIDByDigest(man1.Digest)
	assert.NoError(t, err)
	assert.True(t, found)
	mlIDs, maniDigest, err := sq.Manifest.Remove(man1)
	assert.NoError(t, err)
	assert.Len(t, mlIDs, 0)
	assert.Equal(t, man1.Digest, maniDigest)
	_, found, err = sq.Manifest.GetIDByDigest(man1.Digest)
	assert.NoError(t, err)
	assert.False(t, found)
	_, _, err = sq.Manifest.Remove(man1)
	assert.Error(t, err)

}

func Test_Manifest_GetView(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.AddBootstrap("users")
	bs.AddBootstrap("profiles")
	bs.AddBootstrap("platforms")
	bs.AddBootstrap("upstreams:qnib/featuretest-arm64")
	bs.AddBootstrap("layers:qnib/featuretest-arm64")
	bs.AddBootstrap("config:qnib/featuretest-arm64")
	bs.AddBootstrap("manifest:qnib/featuretest-arm64")
	bs.AddBootstrap("manifestlist:qnib/featuretest-generic-arm64")
	bs.AddBootstrap("collection:qnib/featuretest")
	bs.SetSqlEmptyAll()
	//bs.SetSqlite("Desktop/metahub.db")
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer TeardownTest(bs)
	// Test
	colID, err := br.GetInt64("collection:qnib/featuretest#id")
	assert.NoError(t, err)
	manis, err := sq.Manifest.GetByCollectionId(colID)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(manis))
	expManiID, err := br.GetInt64("manifest:qnib/featuretest-arm64#id")
	assert.NoError(t, err)
	expManiDigest, err := br.Get("manifest:qnib/featuretest-arm64#digest")
	assert.NoError(t, err)
	req := types.Request{
		Repo:   "qnib/featuretest",
		Tag:    "latest",
		Digest: expManiDigest,
	}
	got, found, err := sq.Manifest.GetTypesManifestByRequest(req)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, expManiID, got.SqlID)
	gotManiID, found, err := sq.Manifest.GetIDByDigest(expManiDigest)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, expManiID, gotManiID)
	expMlID, err := br.GetInt64("manifestlist:qnib/featuretest-generic-arm64#id")
	assert.NoError(t, err)
	gotManis2, err := sq.Manifest.TypesManifest_GetByManifestListID(expMlID)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(gotManis2))
	gotManis, err := sq.Manifest.TypesManifest_GetByManifestListID(expMlID)
	assert.NoError(t, err)
	assert.Len(t, gotManis, 1)
	// Delete Manifest
	mlIDs, maniDigest, err := sq.Manifest.Remove(got)
	assert.NoError(t, err)
	assert.Len(t, mlIDs, 1)
	assert.Equal(t, got.Digest, maniDigest)
	_, found, err = sq.Manifest.GetIDByDigest(got.Digest)
	assert.NoError(t, err)
	assert.False(t, found)
	gotManis, err = sq.Manifest.TypesManifest_GetByManifestListID(expMlID)
	assert.NoError(t, err)
	assert.Len(t, gotManis, 0, "Manifest should be deleted. Manifest list has no manifests anymore")

}
