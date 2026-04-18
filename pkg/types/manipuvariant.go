package types

// ManipuVariant binds a rule-expression string to a `ManipuConfig` block.
// A manipulation may have many variants, each selected when its `Rule`
// evaluates true against the target manifest's rules.
type ManipuVariant struct {
	Rule   string       `json:"rule"`
	Config ManipuConfig `json:"config"`
}
