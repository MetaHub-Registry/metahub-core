package types

// ManipuLayer is the in-memory representation of a manipulation-layer
// entry after it has been resolved against the graph. `Select` mirrors
// the YAML field (see pkg/yaml/config/manipulayer.go) and is a
// Python-style slice expression that trims which layers from `Image`
// are appended to the manipulated manifest. An empty string means
// "all layers" for backward compatibility.
type ManipuLayer struct {
	ID          int64
	Label       string
	Image       Manifest
	Rule        string
	Select      string `json:"select,omitempty"`
	Cmd         []ManipuCmd
	Environment []ManipuEnv
}
