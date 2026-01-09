package neo

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestBlobCRUD(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 60
	br, err := SetupTest(ctx, bs)
	_ = br
	if err != nil {
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	// Actual test
	arm_l0 := types.FT_arm_Layer0Reference()
	_, found, err := db.BlobGetByDigest(ctx, arm_l0.Digest.String())
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
	// find blob
	lDig, e := br.Get("layer:0:ft_amd64#digest")
	if e != nil {
		t.Fatal(e)
	}
	_, found, err = db.BlobGetByDigest(ctx, lDig)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	// Delete
	err = db.BlobDeleteByDigest(ctx, lDig)
	if err != nil {
		t.Fatal(err)
	}
	_, found, err = db.BlobGetByDigest(ctx, lDig)
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, found)
}

func TestCreateBlobFromChunk(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	_ = br
	if err != nil {
		t.Fatal(err)
	}
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	defer db.TeardownTest(bs, br)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	// Actual test

	bu := types.OCIBlobUpload{
		UUID:          uuid.New(),
		Repo:          "qnib/featuretest",
		UploadUser:    "test",
		UploadProfile: "generic",
		BlobType:      "layer",
		Digest:        "sha256:6ec0f660682c2f5b391af1d81d3b4839da06f050269546177b28e4b51d8d4c30",
	}
	if err := db.BlobUploadCreateByOCI(ctx, &bu); err != nil {
		t.Fatal(err)
	}
	chunk := types.BlobChunk{
		Digest: "sha256:6ec0f660682c2f5b391af1d81d3b4839da06f050269546177b28e4b51d8d4c30",
		Bytes:  5242880,
	}
	utils.Debug(nil, nil, "> CreateBlobFromChunk")
	if err = db.CreateBlobFromChunk(ctx, &bu, chunk); err != nil {
		t.Fatal(err)
	}
	q := fmt.Sprintf(`MATCH (b:Blob)<-[]-(bu:BlobUpload) WHERE bu.uuid = "%s" RETURN b.digest AS blobDigest`, bu.UUID)
	res, found, err := db.QueryOne(q)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("blob not found")
	}

	blobDigest, err := res.GetStr("blobDigest")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, chunk.Digest, blobDigest)
}
