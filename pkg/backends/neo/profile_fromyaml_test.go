package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

// TestProfileFromYAML_CopiesHardware guards the seed path (`parse profiles`):
// ProfileFromYAML must carry the hardware block from config.Profile into
// types.Profile, otherwise the seeded profile is not model-serving and the
// pull-path gate silently skips model injection (regression caught by the
// model-repackage e2e).
func TestProfileFromYAML_CopiesHardware(t *testing.T) {
	cfgP := config.Profile{
		Name: "frame",
		Rule: "os:linux=true",
		Hardware: &types.Hardware{
			GPU: &types.HardwareGPU{Vendor: "amd", VRAMGB: 124},
		},
	}
	prf, err := db.ProfileFromYAML(context.Background(), cfgP)
	require.NoError(t, err)
	require.NotNil(t, prf.Hardware, "hardware must survive the config→types conversion")
	require.True(t, prf.Hardware.IsModelServing())
	require.Equal(t, 124, prf.Hardware.GPU.VRAMGB)
}
