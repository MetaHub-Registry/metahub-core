package types

// Manipulation is the in-memory representation of a manipulation file.
// It carries both the legacy flat-layers form (`ManipuLayers`) and the
// extended form (`Default`, `Variants`). A round-tripped manipulation
// keeps `ManipuLayers` populated as a flattened view of
// `Default.Layers + variant layers` for backward compatibility with
// existing consumers.
type Manipulation struct {
	Label        string          `json:"label"`
	ID           int64           `json:"id,omitempty"`
	ManipuLayers []ManipuLayer   `json:"layers,omitempty"`
	Default      ManipuConfig    `json:"default,omitempty"`
	Variants     []ManipuVariant `json:"variants,omitempty"`
}

// FlattenLayers returns a single slice of ManipuLayers that combines the
// default layers with every variant's layers, preserving order (default
// first, then variants in slice order).
func (m Manipulation) FlattenLayers() []ManipuLayer {
	out := make([]ManipuLayer, 0, len(m.Default.Layers))
	out = append(out, m.Default.Layers...)
	for _, v := range m.Variants {
		out = append(out, v.Config.Layers...)
	}
	return out
}
