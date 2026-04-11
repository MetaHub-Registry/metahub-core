package types

// OrasArtifact represents a single ORAS artifact manifest with its full metadata.
type OrasArtifact struct {
	ID           int64             `json:"id"`
	Repo         string            `json:"repo"`
	Digest       string            `json:"digest"`
	MediaType    string            `json:"media_type"`
	ArtifactType string            `json:"artifact_type"`
	Annotations  map[string]string `json:"annotations"`
	Subject      *Manifest     `json:"subject,omitempty"`
	Blobs        []Manifest    `json:"blobs"`
}

// OrasListing holds navigation metadata for a single ORAS artifact repo+artifactType entry,
// used for repo-tree traversal (mirrors the role of CollectionListing in collections browsing).
type OrasListing struct {
	Repo          string `json:"repo"`
	ArtifactType  string `json:"artifact_type"`
	ArtifactCount int64  `json:"artifact_count"`
}
