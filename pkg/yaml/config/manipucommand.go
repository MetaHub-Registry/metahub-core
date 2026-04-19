package config

// ManipuCommand carries the desired mutation for a CMD or ENTRYPOINT field of
// an OCI image config. It is part of the extended manipulation schema
// introduced in `extend-manipulation-schema`.
//
// Op is one of the ManipuCmdOp* constants defined in `pkg/types/manipucommand.go`.
type ManipuCommand struct {
	Op    string   `yaml:"op"`
	Value []string `yaml:"value"`
}

// IsZero reports whether the command carries no data and can be omitted from
// the produced graph / DTO.
func (c ManipuCommand) IsZero() bool {
	return c.Op == "" && len(c.Value) == 0
}
