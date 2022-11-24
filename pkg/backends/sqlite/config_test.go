package sqlite

import (
	"fmt"
	"testing"

	"github.com/docker/distribution"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

var (
	testCfg = types.Config{
		Digest:    "sha256:1234567890",
		MediaType: "application/vnd.oci.image.config.v1+json",
	}
)

func Test_Config_Insert(t *testing.T) {
	bs := utils.NewBootstrap()
	defer TeardownTest(bs)
	err := sq.Config.Insert(&testCfg)
	assert.NoError(t, err)
	assert.NotEqual(t, int64(-1), testCfg.SqlID)
}

func Test_Config_Upsert(t *testing.T) {
	bs := utils.NewBootstrap()
	defer TeardownTest(bs)
	err := sq.Config.Upsert(&testCfg)
	assert.NoError(t, err)
	assert.NotEqual(t, int64(-1), testCfg.SqlID)
	tCfg := testCfg
	err = sq.Config.Upsert(&testCfg)
	assert.NoError(t, err)
	assert.Equal(t, tCfg.SqlID, testCfg.SqlID)
}

func Test_Config_GetCreateByRequest(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.AddBootstrap("users")
	bs.AddBootstrap("profiles")
	_, err := SetupTest(bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	// Tests
	expCfg := types.FT_arm_Config()
	ociMani := types.OCIManifest{
		Config: distribution.Descriptor{
			Digest:    digest.Digest(expCfg.Digest),
			MediaType: expCfg.MediaType,
			Size:      expCfg.Size,
		},
	}
	gotCfg, err := sq.Config.GetCreateByRequest(ociMani)
	assert.NoError(t, err)
	assert.NotEqual(t, int64(-1), gotCfg.SqlID)
	gotCfg2, err := sq.Config.GetCreateByRequest(ociMani)
	assert.NoError(t, err)
	assert.Equal(t, gotCfg.SqlID, gotCfg2.SqlID)

}

func Test_Config_gets(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.AddBootstrap("users")
	bs.AddBootstrap("profiles")
	bs.AddBootstrap("config:qnib/featuretest-arm64")
	br, err := SetupTest(bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	// Tests
	expDig, err := br.Get("config:qnib/featuretest-arm64#digest")
	assert.NoError(t, err)
	cfgID, err := sq.Config.GetID(expDig)
	assert.NoError(t, err)
	expCfgID, err := br.Get("config:qnib/featuretest-arm64#id")
	assert.NoError(t, err)
	assert.Equal(t, expCfgID, fmt.Sprint(cfgID))
	cfg := types.Config{
		Digest: expDig,
	}
	found, err := sq.Config.GetByRequest(&cfg)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, expCfgID, fmt.Sprint(cfg.SqlID))
	// Test GetByDigest
	cfg2, found, err := sq.Config.ConfigGetByDigest(expDig)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, expCfgID, fmt.Sprint(cfg2.SqlID))
	err = sq.Config.RemoveByID(cfg2.SqlID)
	assert.NoError(t, err)
	_, found, err = sq.Config.ConfigGetByDigest(expDig)
	assert.NoError(t, err)
	assert.False(t, found)
	err = sq.Config.RemoveByID(cfg2.SqlID)
	assert.Error(t, err)

}
