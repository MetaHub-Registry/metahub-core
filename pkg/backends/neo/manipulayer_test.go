package neo

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

func TestManipuLayerCheckLinks(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("upstream:glow150arm64")
	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("layer:0:glow150arm64_2660")
	bs.AddBootstrap("layer:1:glow150arm64_3571")
	bs.AddBootstrap("layer:2:glow150arm64_d54e")
	bs.AddBootstrap("config:glow150arm64")
	bs.AddBootstrap("manifestdata:glow150arm64")
	bs.AddBootstrap("manipulation:manipulations/layer/glow/1.5.0-arm64.yaml")
	bs.AddBootstrap("profile:profiles/glow/1.5.0-arm64.yaml")
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)
	// Actual test

	ruleLinuxID, err := br.GetInt64("rule:os:linux#id")
	if err != nil {
		t.Fatal(err)
	}
	prfID, err := br.GetInt64("profile:glow:1.5.0-arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	q := fmt.Sprintf(`MATCH (prf:Profile)-[rel]->(r:Rule) WHERE ID(prf) = %d AND ID(r) = %d RETURN count(rel) AS count`, prfID, ruleLinuxID)
	t.Log(q)
	if res, _, err := db.QueryOne(q); err != nil {
		t.Fatal(err)
	} else {
		if cnt, e := res.GetInt("count"); e != nil {
			t.Fatal(e)
		} else {
			assert.Equal(t, 1, cnt)
		}
	}
	// ManipuLayer <> ArchRule
	mplID, err := br.GetInt64("manipulayer:registry:5050/cqnib/metahub-layers/hpc3/glow:1.5.0-arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	ruleArchID, err := br.GetInt64("rule:arch:arm64#id")
	if err != nil {
		t.Fatal(err)
	}
	q = fmt.Sprintf(`MATCH (mpl:ManipuLayer)-[rel]->(r:Rule) WHERE ID(mpl) = %d AND ID(r) = %d RETURN count(rel) AS count`, mplID, ruleArchID)
	t.Log(q)
	if res, _, err := db.QueryOne(q); err != nil {
		t.Fatal(err)
	} else {
		if cnt, e := res.GetInt("count"); e != nil {
			t.Fatal(e)
		} else {
			assert.Equal(t, 1, cnt)
		}
	}

}

func TestManipuLayerFromConfigYAML(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("rules:rules/platform/arch/arm64.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("upstream:glow150arm64")
	bs.AddBootstrap("config:glow150arm64")
	bs.AddBootstrap("layer:0:glow150arm64_2660")
	bs.AddBootstrap("layer:1:glow150arm64_3571")
	bs.AddBootstrap("layer:2:glow150arm64_d54e")
	bs.AddBootstrap("manifestdata:glow150arm64")
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)
	// Actual test
	cfgMp, err := config.LoadManipulation(ctx, filepath.Join("../../../misc/yaml", "manipulations/layer/glow/1.5.0-arm64.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "layer/glow#1.5.0-arm64", cfgMp.Label)
	for _, cfgMpl := range cfgMp.ManipuLayers {
		mpl, err := db.ManipuLayerFromConfigYAML(ctx, cfgMpl)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, "registry:5050/cqnib/metahub-layers/hpc3/glow:1.5.0-arm64", mpl.Label)
	}
}
