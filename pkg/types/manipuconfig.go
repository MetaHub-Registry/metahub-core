package types

// ManipuConfig carries the extended manipulation payload for a
// top-level defaults block OR a rule-keyed variant block.
//
// All fields are optional; `IsZero() == true` when nothing is set.
type ManipuConfig struct {
	ID         int64             `json:"id,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	Workdir    string            `json:"workdir,omitempty"`
	User       string            `json:"user,omitempty"`
	Volumes    []string          `json:"volumes,omitempty"`
	Entrypoint *ManipuCommand    `json:"entrypoint,omitempty"`
	Cmd        *ManipuCommand    `json:"cmd,omitempty"`
	Layers     []ManipuLayer     `json:"layers,omitempty"`
}

// IsZero reports whether the config carries no data.
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
