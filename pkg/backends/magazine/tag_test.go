package magazine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestTagList(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm64")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("collection:ft_generic")
	bs.AddBootstrap("collection:ft_arm64")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 300
	br, err := mag.SetupTest(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs)
	_ = br
	// Test

	req := types.Request{
		Repo: "qnib/featuretest",
	}
	//
	assert.NotNil(t, mag.Database)
	res, err := mag.Database.Collection_GetTagsListByRepo(req.Repo)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 2, len(res.Tags))
	res, err = mag.Tags_GetList(req)
	if err != nil {
		t.Fatal(err)
	}
	// TODO: add setup collection to make this work
	assert.Equal(t, 2, len(res.Tags))
	reqPage := types.Request{
		Repo:   "qnib/featuretest",
		Number: 1,
		Last:   "arm64",
	}
	res, err = mag.Tags_GetList(reqPage)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(res.Tags))
	// Remove Manifests and expect the collection to stick around with 0 tags
	/*m := types.Manifest{
			Upstream: types.Upstream{
				Repo: "qnib-pub-containers/qnib/plain-featuretest",
				Tag:  "20220715-arm64-generic",
			},
			Digest: "sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5",
		}
		_, _, err = mag.SQlite.Manifest.Remove(m)
		if err != nil {
			t.Fatal(err)
	}
		res, err = mag.Tags_GetList(req)
		if err != nil {
			t.Fatal(err)
	}
		assert.Equal(t, 0, len(res.Tags))
	*/
}
