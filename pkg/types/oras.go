package types

// OrasArtifact represents a single ORAS artifact manifest with its full metadata.
type OrasArtifact struct {
	ID           int64             `json:"-"`
	Repo         string            `json:"repo"`
	Digest       string            `json:"digest"`
	MediaType    string            `json:"media_type"`
	ArtifactType string            `json:"artifact_type"`
	Annotations  map[string]string `json:"annotations"`
	Subject      *Manifest         `json:"subject,omitempty"`
	Blobs        []OrasBlob        `json:"blobs"`
}

// OrasBlob represents a single layer blob within an ORAS artifact.
// It carries only the fields relevant to ORAS layers — no Platform,
// Config, or nested Layers (those belong on the parent Manifest).
type OrasBlob struct {
	Digest      string            `json:"digest"`
	MediaType   string            `json:"media_type"`
	Size        int64             `json:"size"`
	Title       string            `json:"title"`
	Annotations map[string]string `json:"annotations"`
}

// OrasListing holds navigation metadata for a single ORAS artifact repo+artifactType entry,
// used for repo-tree traversal (mirrors the role of CollectionListing in collections browsing).
type OrasListing struct {
	Repo          string `json:"repo"`
	ArtifactType  string `json:"artifact_type"`
	ArtifactCount int64  `json:"artifact_count"`
}
