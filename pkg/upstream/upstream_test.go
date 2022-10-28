package upstream

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

var ctx = context.Background()

func Test_Config_GetBlobByUpstreamTriple(t *testing.T) {
	up := UpstreamTriple{
		Reg:    "registry.gitlab.com",
		Repo:   "qnib-pub-containers/qnib/plain-featuretest",
		Tag:    "20220715-arm64-generic",
		Digest: "sha256:cf5c26263929e84d7373d23b2692222cceceb3175b3325b183ead2ab6b5a5ce3",
	}
	b, blobMeta, err := GetBlobByUpstreamTriple(ctx, up)
	assert.NoError(t, err)
	assert.NotNil(t, b)
	assert.NotNil(t, blobMeta)

}
func Test_GetManifestByUpstreamTripleByTag(t *testing.T) {
	up := UpstreamTriple{
		Reg:  "registry.gitlab.com",
		Repo: "qnib-pub-containers/qnib/plain-featuretest",
		Tag:  "20220715-arm64-generic",
	}
	b, blobMeta, err := GetManifestByUpstreamTripleByTag(ctx, up)
	assert.NoError(t, err)
	assert.NotNil(t, b)
	assert.NotNil(t, blobMeta)
	assert.Equal(t, "application/vnd.docker.distribution.manifest.v2+json", blobMeta.MediaType)
	assert.Equal(t, "sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5", blobMeta.Digest.String())
}
