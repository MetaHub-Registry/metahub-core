package types

import (
	"encoding/json"
)

// ManifestWrapper is a legacy "slot-ish" wrapper that represents the
// (host, repo, tag) grouping produced by the pre-Slot graph model. It is
// scheduled for deletion when the `:Slot` node-centred refactor
// (refactor-graph-model-slot-centred §4) introduces a proper Slot type.
// The rename from `Manifest` → `ManifestWrapper` is a temporary step to free
// up the `Manifest` identifier for the renamed content leaf (formerly
// `Manifest`).
type ManifestWrapper struct {
	ID    int64  `json:"-"`
	Label string `json:"label"`
	//Platform    Platform       `json:"platform"`
	Upstream    Upstream       `json:"upstream"`
	Annotations Annotations    `json:"annotations"`
	Manifests   []Manifest `json:"manifests"`
}

func (m ManifestWrapper) ToBytes() (data []byte, err error) {
	data, err = json.Marshal(m)
	return
}

// ManifestWrapperFromManifest constructs a legacy ManifestWrapper wrapper from
// a single Manifest. Deprecated: will be removed when the Slot node
// refactor lands.
func ManifestWrapperFromManifest(md Manifest) (msi ManifestWrapper) {
	msi = ManifestWrapper{
		//Platform:    md.Platform,
		Upstream:    md.Upstream,
		Annotations: md.Annotations,
		Manifests:   []Manifest{md},
	}
	return
}
