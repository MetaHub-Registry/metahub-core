package neo

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_Blobupload_CRU(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 90
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer db.TeardownTest(bs)
	// Actual test
	//// CREATE
	ociBU := types.OCIBlobUpload{
		Repo:          "qnib/featuretest",
		UploadUser:    "test",
		UploadProfile: "generic",
		BlobType:      "layer",
		Digest:        "sha256:1234567890",
	}
	err = db.BlobUpload_Create_ByOCI(ctx, &ociBU)
	assert.NoError(t, err)
	assert.NotNil(t, ociBU.UUID)
	assert.NotNil(t, ociBU.ID)
	assert.NotNil(t, ociBU.Location)
	assert.Regexp(t, fmt.Sprintf(`/v2/%s/blobs/uploads/%s$`, ociBU.Repo, ociBU.UUID), ociBU.Location)
	assert.Equal(t, "layer", ociBU.BlobType)
	assert.Equal(t, "qnib/featuretest", ociBU.Repo)
	//// READ
	ociBU2, found, err := db.BlobUpload_GetOCI_ByUUID(ctx, ociBU.UUID.String())
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, ociBU.ID, ociBU2.ID)
	assert.Equal(t, ociBU.UUID, ociBU2.UUID)
	///// Update
	///////// CONFIG
	cfgDigest, err := br.Get("config:ft_arm64#digest")
	assert.NoError(t, err)
	ociBU.Digest = cfgDigest
	err = db.BlobUpload_Update_ByUUID(ctx, &ociBU)
	assert.NoError(t, err)
	q := `MATCH (bu:BlobUpload)-[:IS]->(c:Config) WHERE id(bu) = %d RETURN ID(c) AS configID`
	query := fmt.Sprintf(q, ociBU.ID)
	res, err := db.Query(query)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res))
	///////// Layer
	layerDigest, err := br.Get("layer:0:ft_arm64#digest")
	assert.NoError(t, err)
	ociBU.Digest = layerDigest
	err = db.BlobUpload_Update_ByUUID(ctx, &ociBU)
	assert.NoError(t, err)
	q = `MATCH (bu:BlobUpload)-[:IS]->(l:Layer) WHERE id(bu) = %d RETURN ID(l) AS ID`
	query = fmt.Sprintf(q, ociBU.ID)
	res, err = db.Query(query)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res))

}

func Test_BlobChunk_CRU(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 90
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer db.TeardownTest(bs)
	// Actual test
	//// CREATE
	ociBU := types.OCIBlobUpload{
		Repo:          "qnib/featuretest",
		UploadUser:    "test",
		UploadProfile: "generic",
		BlobType:      "layer",
		Digest:        "sha256:1234567890",
	}
	err = db.BlobUpload_Create_ByOCI(ctx, &ociBU)
	assert.NoError(t, err)
	bc, err := db.UploadChunk_Create(ctx, ociBU.ID, 0, 10)
	assert.NoError(t, err)
	assert.NotNil(t, bc.ID)
	// / READ
	bcs, err := db.UploadChunk_Get_ByUUID(ctx, ociBU.ID)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(bcs))
}
