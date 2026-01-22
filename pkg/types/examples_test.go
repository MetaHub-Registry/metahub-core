package types

import (
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
)

func TestFTarmConfigByte(t *testing.T) {
	dat := ConfigByteFtArm64()
	assert.Len(t, dat, 2191)
	assert.Equal(t, "sha256:cf5c26263929e84d7373d23b2692222cceceb3175b3325b183ead2ab6b5a5ce3", digest.FromBytes(dat).String())
}

func TestFTarmManifest(t *testing.T) {
	mani := FTarmManifestData()
	assert.Equal(t, "sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5", mani.Digest)
	assert.Len(t, mani.Layers, 2)
}

func TestFTarmLayerReference(t *testing.T) {
	dat := FTarmLayer0Reference()
	assert.Equal(t, int64(2693535), dat.Size)
	dat = FTarmLayer1Reference()
	assert.Equal(t, int64(304), dat.Size)
}

func TestFTarmLayerBytes(t *testing.T) {
	dat := FTarmLayer0Byte()
	assert.Equal(t, 2693535, len(dat))
	dat = FTarmLayer1Byte()
	assert.Equal(t, 304, len(dat))
}
