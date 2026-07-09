package types

// ModelProfileConfig is the runtime configuration a publisher pushed for a model
// under a specific hardware profile — the `config/<name>` manipulation produced
// by oras-model-config-injection. It links the profile to the config image
// (`models/<name>-config:<quant>`) and the in-image directory the base image
// reads the config from (`QNIB_MODEL_CONFIG_DIR`).
type ModelProfileConfig struct {
	Profile   string `json:"profile"`              // profile label, e.g. vram#12gb
	Repo      string `json:"repo,omitempty"`       // config image repo, models/<name>-config
	Tag       string `json:"tag,omitempty"`        // quantization tag
	Digest    string `json:"digest,omitempty"`     // config image manifest digest
	ConfigDir string `json:"config_dir,omitempty"` // QNIB_MODEL_CONFIG_DIR the base image reads
}

// ModelDetail is the card payload for a single model (GET /api/v1/models/:ref):
// every quantization grouped under the one model reference (each with its size
// and repackaged layer image), the per-profile runtime configs, the profiles the
// model is served under, and the source ORAS artifact. It is the detail
// counterpart of ModelListing.
type ModelDetail struct {
	Name           string               `json:"name"`                      // <name> from the model/<name> label
	Label          string               `json:"label"`                     // full manipulation label, model/<name>
	Profiles       []string             `json:"profiles"`                  // profile labels that inject the model
	Quantizations  []ModelImageRef      `json:"quantizations"`             // every quant (layer image + size) under this model
	Configs        []ModelProfileConfig `json:"configs"`                   // per-profile runtime configs the publisher pushed
	Compositions   []ModelComposition   `json:"compositions"`              // runnable bundles auto-derived from referrers(model)
	TotalSize      int64                `json:"total_size,omitempty"`      // sum of the distinct quantizations' sizes
	SourceArtifact *ModelSourceArtifact `json:"source_artifact,omitempty"` // source ORAS artifact via OCI subject ref
}
