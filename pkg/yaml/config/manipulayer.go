package config

// ManipuLayer carries a single entry under a manipulation's `layers:`
// list. `Select` is optional — when set it is a Python-style slice
// expression ("[1:]", "[0]", "[-1]", "[1:3]") that picks which layers
// from the source image are appended to the manipulated manifest.
// Parsing / validation happens in metalib/pkg/utils.ParseLayerSlice;
// an empty string is treated as "all layers" to keep existing YAMLs
// working unchanged.
type ManipuLayer struct {
	Image       string `yaml:"image"`
	Rule        string `yaml:"rule"`
	Select      string `yaml:"select,omitempty"`
	Cmd         []Cmd
	Environment []Environment
}
