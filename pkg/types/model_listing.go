package types

// ModelImageRef is one repackaged model-layer image variant of a model — the
// OCI image `models/<name>-layer:<quant>` that `repackage` produced and that a
// `model/<name>` manipulation injects into a base image at pull time.
type ModelImageRef struct {
	Repo   string `json:"repo"`           // e.g. models/qwen3-8b-layer
	Tag    string `json:"tag"`            // quantization tag, e.g. q8-kxl
	Digest string `json:"digest"`         // model-layer manifest digest
	Size   int64  `json:"size,omitempty"` // model-layer tar blob size in bytes (the model weight)
}

// ModelSourceArtifact is the source ORAS model artifact a model was repackaged
// from, linked via the model-layer image's OCI subject reference (the
// model-layer Manifest -[:SUBJECT_OF]-> source Manifest edge metahub wires on
// push). Absent for models repackaged before subject references were emitted.
type ModelSourceArtifact struct {
	Repo   string `json:"repo"`   // e.g. e2e-models/e2e-gguf35
	Digest string `json:"digest"` // source artifact manifest digest
}

// ModelListing is a catalog entry for one model. A model is a `model/<name>`
// manipulation: the profiles that inject it (Profile-[:USES]->Manipulation) and
// its repackaged model-layer image variant(s)
// (Manipulation-[:HAS]->ManipuLayer-[:UPSTREAM]->Upstream). It is the Models-tab
// analogue of CollectionListing/OrasListing.
type ModelListing struct {
	Name           string               `json:"name"`                      // <name> from the model/<name> label
	Label          string               `json:"label"`                     // full manipulation label, model/<name>
	Profiles       []string             `json:"profiles"`                  // VRAM profile labels that inject the model
	Images         []ModelImageRef      `json:"images"`                    // repackaged model-layer image variant(s)
	TotalSize      int64                `json:"total_size,omitempty"`      // sum of the distinct quantizations' model-layer sizes
	SourceArtifact *ModelSourceArtifact `json:"source_artifact,omitempty"` // source ORAS artifact via OCI subject ref
}
