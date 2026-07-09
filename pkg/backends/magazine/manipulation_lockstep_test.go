package magazine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestValidateLayerDiffIDLockstep covers the compose-time guard that turns the
// cryptic client-side "number of layers and diffIDs don't match" pull failure
// into an attributed server-side error at the offending image.
func TestValidateLayerDiffIDLockstep(t *testing.T) {
	// Matching counts (including the empty image) are the valid case.
	assert.NoError(t, validateLayerDiffIDLockstep("original manifest 1", 3, 3))
	assert.NoError(t, validateLayerDiffIDLockstep("manipulated manifest", 0, 0))

	// The canonical corruption seen on the bazzite pull: 14 manifest layers vs
	// 11 config DiffIDs. The error must name the image and echo both the
	// human-readable counts and the exact docker-facing string.
	err := validateLayerDiffIDLockstep("original manifest 42", 14, 11)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "original manifest 42")
	assert.Contains(t, err.Error(), "14 layers but 11 config DiffIDs")
	assert.Contains(t, err.Error(), "14 != 11")

	// The reverse drift (fewer layers than DiffIDs) is caught too, with the
	// ManipuLayer attribution string.
	err2 := validateLayerDiffIDLockstep("ManipuLayer image 7 (config/foo)", 2, 5)
	assert.Error(t, err2)
	assert.Contains(t, err2.Error(), "ManipuLayer image 7 (config/foo)")
	assert.Contains(t, err2.Error(), "2 != 5")
}
