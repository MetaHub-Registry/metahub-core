package types_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func TestOrasListingJSONRoundTrip(t *testing.T) {
	original := types.OrasListing{
		Repo:          "qnib/featuretest",
		ArtifactType:  "application/vnd.cyclonedx",
		ArtifactCount: 3,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded types.OrasListing
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.Repo, decoded.Repo)
	assert.Equal(t, original.ArtifactType, decoded.ArtifactType)
	assert.Equal(t, original.ArtifactCount, decoded.ArtifactCount)
}

func TestOrasArtifactJSONRoundTrip(t *testing.T) {
	original := types.OrasArtifact{
		ID:           42,
		Repo:         "qnib/featuretest",
		Digest:       "sha256:abc123",
		MediaType:    "application/vnd.oci.image.manifest.v1+json",
		ArtifactType: "application/vnd.cyclonedx",
		Annotations:  map[string]string{"org.example.key": "value"},
		Blobs:        []types.Manifest{},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded types.OrasArtifact
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.ID, decoded.ID)
	assert.Equal(t, original.Repo, decoded.Repo)
	assert.Equal(t, original.Digest, decoded.Digest)
	assert.Equal(t, original.MediaType, decoded.MediaType)
	assert.Equal(t, original.ArtifactType, decoded.ArtifactType)
	assert.Equal(t, original.Annotations, decoded.Annotations)
	assert.Nil(t, decoded.Subject)
}

func TestOrasArtifactJSONFields(t *testing.T) {
	a := types.OrasArtifact{
		ID:           1,
		Repo:         "repo",
		Digest:       "sha256:deadbeef",
		MediaType:    "application/vnd.oci.image.manifest.v1+json",
		ArtifactType: "application/vnd.spdx+json",
		Annotations:  map[string]string{},
		Blobs:        []types.Manifest{},
	}
	data, err := json.Marshal(a)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	assert.Contains(t, m, "id")
	assert.Contains(t, m, "repo")
	assert.Contains(t, m, "digest")
	assert.Contains(t, m, "media_type")
	assert.Contains(t, m, "artifact_type")
	assert.Contains(t, m, "annotations")
	assert.Contains(t, m, "blobs")
	// subject is omitempty — should not appear when nil
	assert.NotContains(t, m, "subject")
}
