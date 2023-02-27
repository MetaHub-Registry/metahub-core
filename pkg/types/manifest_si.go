package types

import (
	"encoding/json"
)

type Manifest struct {
	ID          int64          `json:"-"`
	Platform    Platform       `json:"platform"`
	Upstream    Upstream       `json:"upstream"`
	Annotations Annotations    `json:"annotations"`
	Manifests   []ManifestData `json:"manifests"`
}

func (m Manifest) ToBytes() (data []byte, err error) {
	data, err = json.Marshal(m)
	return
}

func ManifestFromManifestData(md ManifestData) (msi Manifest) {
	msi = Manifest{
		Platform:    md.Platform,
		Upstream:    md.Upstream,
		Annotations: md.Annotations,
		Manifests:   []ManifestData{md},
	}
	return
}
