package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// ModelManipulationLabel is the manipulation label for a model: `model/<name>`.
func ModelManipulationLabel(name string) string {
	return "model/" + name
}

// ConfigManipulationLabel is the manipulation label for a model's runtime config:
// `config/<name>`. Sibling of ModelManipulationLabel so the model layer and the
// per-profile config layer are versioned independently but both attach to the
// profile (a pull under it injects both).
func ConfigManipulationLabel(name string) string {
	return "config/" + name
}

// VRAMVariantKey is the variant rule key for a VRAM tier: `vram:<tier>=true`
// (e.g. `vram:12gb=true`). A node profile presenting that attribute selects the
// matching model-layer image at pull time via the existing variant engine.
func VRAMVariantKey(tier string) string {
	return fmt.Sprintf("vram:%s=true", tier)
}

// UpsertModelVariant loads a manipulation from yamlIn (or starts fresh when it is
// empty), sets the `model/<name>` label, and adds or replaces the
// `vram:<tier>=true` variant so it references exactly the given model-layer
// image. Existing variants for other tiers are preserved untouched. It returns
// the re-marshalled YAML.
//
// This is the read-modify-write the repackage step performs on
// `manipulations/<name>/manipulation.yaml`; callers guard concurrent writers
// (design Q11).
func UpsertModelVariant(yamlIn []byte, name, tier, image string) ([]byte, error) {
	var m Manipulation
	if len(yamlIn) > 0 {
		if err := yaml.Unmarshal(yamlIn, &m); err != nil {
			return nil, fmt.Errorf("parse manipulation yaml: %w", err)
		}
	}
	m.Label = ModelManipulationLabel(name)
	if m.Variants == nil {
		m.Variants = map[string]ManipuConfig{}
	}
	m.Variants[VRAMVariantKey(tier)] = ManipuConfig{
		Layers: []ManipuLayer{{Image: image}},
	}
	out, err := yaml.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshal manipulation yaml: %w", err)
	}
	return out, nil
}
