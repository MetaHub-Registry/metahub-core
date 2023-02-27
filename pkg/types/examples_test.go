package types

import (
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
)

func Test_FT_arm_ConfigByte(t *testing.T) {
	dat := FT_arm_ConfigByte()
	assert.Len(t, dat, 2191)
	assert.Equal(t, "sha256:cf5c26263929e84d7373d23b2692222cceceb3175b3325b183ead2ab6b5a5ce3", digest.FromBytes(dat).String())
}

func Test_FT_arm_Manifest(t *testing.T) {
	mani := FT_arm_ManifestData()
	assert.Equal(t, "sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5", mani.Digest)
	assert.Len(t, mani.Layers, 2)
}

func Test_FT_arm_LayerReference(t *testing.T) {
	dat := FT_arm_Layer0Reference()
	assert.Equal(t, int64(2693535), dat.Size)
	dat = FT_arm_Layer1Reference()
	assert.Equal(t, int64(304), dat.Size)
}

func Test_FT_arm_LayerBytes(t *testing.T) {
	dat := FT_arm_Layer0Byte()
	assert.Equal(t, 2693535, len(dat))
	dat = FT_arm_Layer1Byte()
	assert.Equal(t, 304, len(dat))
}
