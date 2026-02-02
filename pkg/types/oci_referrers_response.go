package types

import (
	"encoding/json"
	"net/http"

	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

type ReferrersResponse struct {
	SchemaVersion int                    `json:"schemaVersion"`
	MediaType     string                 `json:"mediaType"`
	Manifests     []imgspecv1.Descriptor `json:"manifests"`
}

func (r ReferrersResponse) WriteWWW(w http.ResponseWriter) {
	w.Header().Set("Content-Type", r.MediaType)
	b, err := r.ToBytes()
	if err != nil {
		logrus.Errorf("failed to marshal manifest: %v", err)
		return
	}
	w.Write(b)
}

func (r ReferrersResponse) ToBytes() (data []byte, err error) {
	return json.Marshal(r)
}

func NewReferrersResponse() ReferrersResponse {
	return ReferrersResponse{
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.index.v1+json",
		Manifests:     []imgspecv1.Descriptor{},
	}
}

func RefferrersResponseFromBytes(data []byte) (r ReferrersResponse, err error) {
	err = json.Unmarshal(data, &r)
	return
}
