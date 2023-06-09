package neo

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestBlobuploadCRU(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 90
	br, err := SetupTest(ctx, bs)
	_ = br
	if err != nil {
		t.Fatal(err)
	}
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
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, ociBU.UUID)
	assert.NotNil(t, ociBU.ID)
	assert.NotNil(t, ociBU.Location)
	assert.Regexp(t, fmt.Sprintf(`/v2/%s/blobs/uploads/%s$`, ociBU.Repo, ociBU.UUID), ociBU.Location)
	assert.Equal(t, "layer", ociBU.BlobType)
	assert.Equal(t, "qnib/featuretest", ociBU.Repo)
	//// READ
	ociBU2, found, err := db.BlobUpload_GetOCI_ByUUID(ctx, ociBU.UUID.String())
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, ociBU.ID, ociBU2.ID)
	assert.Equal(t, ociBU.UUID, ociBU2.UUID)
	///// Update
	///////// CONFIG
	cfgDigest, err := br.Get("config:ft_arm64#digest")
	if err != nil {
		t.Fatal(err)
	}
	ociBU.Digest = cfgDigest
	err = db.BlobUpload_Update_ByUUID(ctx, &ociBU)
	if err != nil {
		t.Fatal(err)
	}
	q := `MATCH (bu:BlobUpload)-[:UPLOADED]->(c:Config) WHERE id(bu) = %d RETURN ID(c) AS configID`
	query := fmt.Sprintf(q, ociBU.ID)
	res, err := db.Query(query)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(res))
	///////// Layer
	layerDigest, err := br.Get("layer:0:ft_arm64#digest")
	if err != nil {
		t.Fatal(err)
	}
	ociBU.Digest = layerDigest
	err = db.BlobUpload_Update_ByUUID(ctx, &ociBU)
	if err != nil {
		t.Fatal(err)
	}
	q = `MATCH (bu:BlobUpload)-[:UPLOADED]->(l:Layer) WHERE id(bu) = %d RETURN ID(l) AS ID`
	query = fmt.Sprintf(q, ociBU.ID)
	res, err = db.Query(query)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(res))
}

func TestBlobChunkCRU(t *testing.T) {
	ctx := context.Background()
	debug := 0
	bs := utils.NewBootstrap()
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs)
		t.Fatal(err)
	}
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	defer db.TeardownTest(bs)
	// Actual test
	//// CREATE
	ociBU := types.OCIBlobUpload{
		Repo:          "qnib/featuretest",
		UploadUser:    "test",
		UploadProfile: "generic",
		BlobType:      "layer",
		Digest:        "sha256:6ec0f660682c2f5b391af1d81d3b4839da06f050269546177b28e4b51d8d4c30",
	}
	err = db.BlobUpload_Create_ByOCI(ctx, &ociBU)
	if err != nil {
		t.Fatal(err)
	}
	bc, err := db.UploadChunk_Create(ctx, ociBU.ID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, bc.ID)
	// / READ
	bcs, err := db.UploadChunk_Get_ByUUID(ctx, ociBU.ID)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(bcs))
	// LinkBlobUpdate
	if e := db.BlobUpload_Update_ByUUID(ctx, &ociBU); e != nil {
		t.Fatal(e)
	}
	// BlobUploadGetByBlobDigest
	if gotBu, found, err := db.BlobUploadGetByBlobDigest(ctx, ociBU.Digest); err != nil {
		t.Fatal(err)
	} else if !found {
		t.Fatal("Did not find BlobUpload")
	} else {
		assert.Equal(t, ociBU.ID, gotBu.ID)
	}
}
