package upstream

import (
	"context"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
)

var ctx = context.Background()

func TestConfigGetBlobByUpstreamTriple(t *testing.T) {
	up := UpstreamTriple{
		Reg:    regHttpURI,
		Repo:   "alpine",
		Tag:    "3.17.3",
		Digest: "sha256:51e60588ff2cd9f45792b23de89bfface0a7fbd711d17c5f5ce900a4f6b16260",
	}
	b, blobMeta, err := GetBlobByUpstreamTriple(ctx, up)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, b)
	assert.NotNil(t, blobMeta)
}

func TestGetBlobByUpstreamTripleByDigest(t *testing.T) {
	up := UpstreamTriple{
		Reg:    regHttpURI,
		Repo:   "alpine",
		Tag:    "3.17.2",
		Digest: digest.Digest("sha256:ff6bdca1701f3a8a67e328815ff2346b0e4067d32ec36b7992c1fdc001dc8517"),
	}
	b, blob, err := GetManifestByUpstreamTripleByTag(ctx, up)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, b)
	assert.NotNil(t, blob)
	//t.Log(string(b))
	assert.Equal(t, "sha256:ff6bdca1701f3a8a67e328815ff2346b0e4067d32ec36b7992c1fdc001dc8517", digest.FromBytes(b).String())
}

func TestGetManifestByUpstreamTripleByTag(t *testing.T) {
	up := UpstreamTriple{
		Reg:  regHttpURI,
		Repo: "alpine",
		Tag:  "3.17.3",
	}
	b, blobMeta, err := GetManifestByUpstreamTripleByTag(ctx, up)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, b)
	assert.NotNil(t, blobMeta)
	assert.Equal(t, "application/vnd.docker.distribution.manifest.v2+json", blobMeta.MediaType)
	assert.Equal(t, "sha256:b5a5b7ce4eabc8414bf367761a28f4e8b16952ce5de537c15ed917b71b245f11", blobMeta.Digest.String())
}

func TestGetManifestByUpstreamTripleByDigest(t *testing.T) {
	up := UpstreamTriple{
		Reg:    regHttpURI,
		Repo:   "alpine",
		Tag:    "3.17.3",
		Digest: "sha256:b5a5b7ce4eabc8414bf367761a28f4e8b16952ce5de537c15ed917b71b245f11",
	}
	b, blobMeta, err := GetManifestByUpstreamTripleByDigest(ctx, up)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, b)
	assert.NotNil(t, blobMeta)
	assert.Equal(t, "application/vnd.docker.distribution.manifest.v2+json", blobMeta.MediaType)
	assert.Equal(t, "sha256:b5a5b7ce4eabc8414bf367761a28f4e8b16952ce5de537c15ed917b71b245f11", blobMeta.Digest.String())
}

func TestHeadManifestByUpstreamTripleByTag(t *testing.T) {
	ctx := context.Background()

	blobMeta, found, err := HeadManifestByUpstreamTripleByTag(ctx, regHttpURI, "alpine", "3.17.3")
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.NotNil(t, blobMeta)
	assert.Equal(t, "application/vnd.docker.distribution.manifest.v2+json", blobMeta.MediaType)
	assert.Equal(t, "sha256:b5a5b7ce4eabc8414bf367761a28f4e8b16952ce5de537c15ed917b71b245f11", blobMeta.Digest.String())

}
