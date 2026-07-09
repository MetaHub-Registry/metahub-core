package types

// Composition annotation keys carried on a config artefact (Decision A): a config
// is a sibling ORAS artefact `subject`→the model-layer image it composes with,
// and its annotations identify the composition point — which profile it targets,
// which runtime (application container) it is meant for, and which quantization it
// pairs with. `referrers(model)` + these annotations assemble a composition
// without bespoke graph traversal.
const (
	// ModelProfileAnnotation is the hardware/intent profile the config targets.
	ModelProfileAnnotation = "org.qnib.model.profile"
	// ModelQuantAnnotation is the quantization the config pairs with.
	ModelQuantAnnotation = "org.qnib.model.quant"
	// ModelRuntimeAnnotation names the runtime (application container) the config
	// is meant for; empty when the publisher did not pin one.
	ModelRuntimeAnnotation = "org.qnib.model.runtime"
)

// ModelComposition is a runnable bundle auto-derived from a model's referrers: a
// config artefact (`subject`→a model-layer quant) contributes its
// `{profile, runtime, quant}` annotations plus the config image it repackages
// into. The runtime is the application container name the config names; resolving
// it to a concrete image is the Runtime registry (later). Assembled purely from
// `referrers(model)` + annotations — no bespoke graph traversal.
type ModelComposition struct {
	Profile      string `json:"profile,omitempty"`       // hardware/intent profile the config targets
	Quant        string `json:"quant,omitempty"`         // quantization this composition serves
	Runtime      string `json:"runtime,omitempty"`       // application container name the config composes with
	RuntimeImage string `json:"runtime_image,omitempty"` // resolved runtime image ref (from the registry), if registered
	ConfigRepo   string `json:"config_repo,omitempty"`   // config image repo, models/<name>-config
	ConfigTag    string `json:"config_tag,omitempty"`    // config image tag
	ConfigDigest string `json:"config_digest,omitempty"` // config artefact manifest digest
}

// ConfigCompositionAnnotations builds the annotation map a config artefact
// carries. Empty values are omitted so the manifest stays minimal.
func ConfigCompositionAnnotations(profile, quant, runtime string) map[string]string {
	a := map[string]string{}
	if profile != "" {
		a[ModelProfileAnnotation] = profile
	}
	if quant != "" {
		a[ModelQuantAnnotation] = quant
	}
	if runtime != "" {
		a[ModelRuntimeAnnotation] = runtime
	}
	return a
}
