package magazine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func Test_Collection_ComputeManifest(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:generic")
	bs.AddBootstrap("rules:rules/mpi/mpich.yaml")
	bs.AddBootstrap("rules:rules/cpu/generic.yaml")
	bs.AddBootstrap("annotation:org.supercontainers.hardware.cpu.target:platform")
	bs.AddBootstrap("annotation:org.supercontainers.mpi.implementation:mpich")
	bs.AddBootstrap("profile:profiles/mpi/mpich/generic.yaml")
	bs.AddBootstrap("profile:profiles/cpu/generic.yaml")
	bs.AddBootstrap("config:ft_arm")
	bs.AddBootstrap("config:ft_amd64")
	bs.AddBootstrap("upstream:ft_arm64")
	bs.AddBootstrap("upstream:ft_amd64")
	bs.AddBootstrap("manifest:ft_arm64")
	bs.AddBootstrap("manifest:ft_amd64")
	bs.AddBootstrap("manifest:ft_mpich_amd64")
	bs.AddBootstrap("layer:0:ft_arm64")
	bs.AddBootstrap("layer:1:ft_arm64")
	bs.AddBootstrap("layer:0:ft_amd64")
	bs.AddBootstrap("layer:1:ft_amd64")
	bs.AddBootstrap("manifestdata:ft_arm64")
	bs.AddBootstrap("manifestdata:ft_amd64")
	bs.AddBootstrap("collection:ft_generic")
	bs.SetEmptyAll()
	//bs.PostTestSleep = 300
	br, err := mag.SetupTest(ctx, bs)
	_ = br
	//br.PrintDict()
	if err != nil {
		mag.Database.TeardownTest(bs)
		t.Fatal(err)
	}
	defer mag.Database.TeardownTest(bs)
	// Make more links
	mdArmID, err := br.GetInt64("manifestdata:ft_arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	aMpichID, err := br.GetInt64("annotation:org.supercontainers.mpi.implementation:mpich#id")
	if err != nil {
		t.Fatal(err)
	}

	err = mag.Database.ManifestData_LinkToAnnotation(ctx, mdArmID, aMpichID)
	if err != nil {
		t.Fatal(err)
	}
	// Actual test
	err = mag.ComputeManifests(ctx)
	assert.NoError(t, err)
	err = mag.Database.Rule_EvalAll(ctx)
	assert.NoError(t, err)
	// Get By Rule
	r, found, err := mag.Database.Rule_GetByLabel(ctx, "cpu:generic")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Rule not found")
	}
	mds, err := mag.Database.ManifestData_GetByRule(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 2, len(mds))
	// MPICH Rule
	// Get By Rule
	rMpiGen, found, err := mag.Database.Rule_GetByLabel(ctx, "mpi:mpich")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("Rule not found")
	}
	mdsMpiGen, err := mag.Database.ManifestData_GetByRule(ctx, rMpiGen)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(mdsMpiGen), mdsMpiGen)
}
