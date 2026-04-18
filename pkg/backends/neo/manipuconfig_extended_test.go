package neo

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

// TestManipulationFromConfig_ExtendedSchemaNoLayers verifies the new-form
// path of `ManipulationFromConfigManipulation` handles a YAML config that
// has defaults + variants but no image layers (no upstream resolution
// required).
func TestManipulationFromConfig_ExtendedSchemaNoLayers(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	cfgMp := config.Manipulation{
		Label: "layer/extended-noimg#1.0.0",
		// Top-level defaults (new form, no layers with rule).
		Workdir: "/opt/tool",
		User:    "1000:1000",
		Env:     map[string]string{"APP_HOME": "/opt/tool"},
		Entrypoint: &config.ManipuCommand{
			Op:    "replace",
			Value: []string{"/opt/tool/bin/run"},
		},
		Variants: map[string]config.ManipuConfig{
			"arch:arm64=true": {
				Env: map[string]string{"ARCH": "arm64"},
			},
			"arch:amd64=true": {
				Env: map[string]string{"ARCH": "amd64"},
				Cmd: &config.ManipuCommand{Op: "append", Value: []string{"--avx"}},
			},
		},
	}
	mp, err := db.ManipulationFromConfigManipulation(ctx, cfgMp)
	require.NoError(t, err)
	assert.Equal(t, "layer/extended-noimg#1.0.0", mp.Label)
	assert.Equal(t, "/opt/tool", mp.Default.Workdir)
	assert.Equal(t, "1000:1000", mp.Default.User)
	require.NotNil(t, mp.Default.Entrypoint)
	assert.Equal(t, "replace", mp.Default.Entrypoint.Op)
	require.Len(t, mp.Variants, 2)
	// Persist it and read back.
	require.NoError(t, db.ManipulationCreate(ctx, &mp))
	got, found, err := db.ManipulationGetByID(ctx, mp.ID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "/opt/tool", got.Default.Workdir)
	require.Len(t, got.Variants, 2)
}

// TestManipulationLinkToLayerDirectly covers the manipuConfigLinkLayer
// helper by wiring a ManipuConfig to a freshly-created ManipuLayer node
// without going through the Upstream pipeline.
func TestManipulationConfigLinkLayerDirectly(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	if err != nil {
		db.TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)

	// Create a bare ManipuLayer directly (no Upstream/Manifest linkage).
	res, err := db.Execute(`CREATE (mpl:ManipuLayer {label: "direct-lyr", rule: ""}) RETURN id(mpl) AS mplID`)
	require.NoError(t, err)
	mplID, err := res.GetInt64("mplID")
	require.NoError(t, err)

	// Create a ManipuConfig.
	res2, err := db.Execute(`CREATE (cfg:ManipuConfig {env: "", workdir: "/x", user: "", volumes: ""}) RETURN id(cfg) AS cfgID`)
	require.NoError(t, err)
	cfgID, err := res2.GetInt64("cfgID")
	require.NoError(t, err)

	// Link them.
	require.NoError(t, db.manipuConfigLinkLayer(ctx, cfgID, mplID, 3))

	// Verify the edge exists by direct Cypher — `manipuConfigLayers`
	// requires the ManipuLayer to have a linked Manifest and rule, which
	// this direct test deliberately skips.
	q := fmt.Sprintf(`MATCH (cfg:ManipuConfig)-[r:HAS_LAYER]->(mpl:ManipuLayer) WHERE id(cfg) = %d RETURN r.order AS ord, mpl.label AS lbl`, cfgID)
	check, _, err := db.QueryOne(q)
	require.NoError(t, err)
	ord, _ := check.GetInt64("ord")
	lbl, _ := check.GetStr("lbl")
	assert.Equal(t, int64(3), ord)
	assert.Equal(t, "direct-lyr", lbl)
}
