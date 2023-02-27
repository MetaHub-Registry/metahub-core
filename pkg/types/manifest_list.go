package types

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/docker/distribution/manifest/manifestlist"
)

type ManifestList struct {
	ID            int64          `json:"id"`
	Size          int            `json:"size"`
	Digest        string         `json:"digest"`
	SchemaVersion int            `json:"schema_version"`
	MediaType     string         `json:"media_type"`
	Manifests     []ManifestData `json:"manifests"`
}

type OCIManifestList struct {
	SchemaVersion int                               `json:"schemaVersion"`
	MediaType     string                            `json:"mediaType"`
	Manifests     []manifestlist.ManifestDescriptor `json:"manifests"`
}

func (ml ManifestList) ToOCIBytes() (b []byte) {
	oci := OCIManifestList{
		SchemaVersion: ml.SchemaVersion,
		MediaType:     ml.MediaType,
		Manifests:     []manifestlist.ManifestDescriptor{},
	}
	// Sort by digest
	digList := []string{}
	digMap := map[string]ManifestData{}
	for _, m := range ml.Manifests {
		digList = append(digList, m.Digest)
		digMap[m.Digest] = m
	}
	sort.Strings(digList)
	for _, d := range digList {
		m := digMap[d]
		oci.Manifests = append(oci.Manifests, m.ToOCIManifestDescriptor())
	}
	b, _ = json.Marshal(oci)
	return
}

func (ml ManifestList) WriteWWWHeader(w http.ResponseWriter) {
	// TODO: Not sure if that is stable over time. We might need to
	//
	b := ml.ToOCIBytes()
	size := len(b)
	w.Header().Set("Content-Type", ml.MediaType)
	w.Header().Set("Content-Length", fmt.Sprint(size))
	w.Header().Set("Docker-Content-Digest", ml.Digest)
	w.Header().Set("Etag", ml.Digest)
}

func (ml ManifestList) WriteWWW(w http.ResponseWriter) {
	b := ml.ToOCIBytes()
	size := len(b)
	w.Header().Set("Content-Type", ml.MediaType)
	w.Header().Set("Content-Length", fmt.Sprint(size))
	w.Header().Set("Docker-Content-Digest", ml.Digest)
	w.Header().Set("Etag", ml.Digest)
	w.Write(b)
}
