package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestRuntimeUpsertAndGetAll verifies runtime registration is idempotent on name
// (a second upsert updates, not duplicates), formats round-trip through the CSV
// scalar, RuntimesGetAll returns all, and unsafe input is rejected.
func TestRuntimeUpsertAndGetAll(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	require.NoError(t, err)
	defer db.TeardownTest(bs, br)

	id1, err := db.RuntimeUpsert(ctx, types.Runtime{
		Name: "llama-server", Image: "reg/llama:1", Formats: []string{"gguf"}, ConfigContract: "qnib.model.config.v1",
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, id1, int64(0)) // Neo4j internal ids start at 0

	// Same name → update in place (idempotent on name).
	id2, err := db.RuntimeUpsert(ctx, types.Runtime{
		Name: "llama-server", Image: "reg/llama:2", Formats: []string{"gguf", "ollama"}, ConfigContract: "",
	})
	require.NoError(t, err)
	require.Equal(t, id1, id2)

	_, err = db.RuntimeUpsert(ctx, types.Runtime{Name: "vllm", Image: "reg/vllm:1", Formats: []string{"safetensors"}})
	require.NoError(t, err)

	rts, err := db.RuntimesGetAll(ctx)
	require.NoError(t, err)
	require.Len(t, rts, 2)

	byName := map[string]types.Runtime{}
	for _, rt := range rts {
		byName[rt.Name] = rt
	}
	llama := byName["llama-server"]
	assert.Equal(t, "reg/llama:2", llama.Image, "second upsert updates the image")
	assert.Equal(t, []string{"gguf", "ollama"}, llama.Formats)
	assert.Equal(t, "", llama.ConfigContract)
	assert.Equal(t, []string{"safetensors"}, byName["vllm"].Formats)

	// Compatibility over the stored records.
	assert.True(t, types.RuntimeCompatible(byName["llama-server"], "gguf", "anything"), "empty contract = wildcard")
	assert.False(t, types.RuntimeCompatible(byName["vllm"], "gguf", ""))

	// Unsafe name rejected.
	_, err = db.RuntimeUpsert(ctx, types.Runtime{Name: `bad" name`, Image: "x"})
	require.Error(t, err)
}
