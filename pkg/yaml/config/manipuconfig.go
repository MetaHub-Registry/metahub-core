package config

// ManipuConfig is the new extended manipulation block used by both the
// top-level defaults and each rule-keyed variant. See
// `openspec/changes/extend-manipulation-schema/design.md` for the schema
// rationale.
//
// All fields are optional. The parser treats a config block as empty when
// `IsZero() == true`.
type ManipuConfig struct {
	Env        map[string]string `yaml:"env,omitempty"`
	Workdir    string            `yaml:"workdir,omitempty"`
	User       string            `yaml:"user,omitempty"`
	Volumes    []string          `yaml:"volumes,omitempty"`
	Entrypoint *ManipuCommand    `yaml:"entrypoint,omitempty"`
	Cmd        *ManipuCommand    `yaml:"cmd,omitempty"`
	Layers     []ManipuLayer     `yaml:"layers,omitempty"`
}

// IsZero reports whether every field is unset. Used by the parser to decide
// whether to emit a `Default` block at all.
func (c ManipuConfig) IsZero() bool {
	if len(c.Env) != 0 || c.Workdir != "" || c.User != "" || len(c.Volumes) != 0 || len(c.Layers) != 0 {
		return false
	}
	if c.Entrypoint != nil && !c.Entrypoint.IsZero() {
		return false
	}
	if c.Cmd != nil && !c.Cmd.IsZero() {
		return false
	}
	return true
}
