package types

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type Index struct {
	ID                  int64                 `json:"id"`
	Size                int                   `json:"size"`
	Digest              string                `json:"digest"`
	SchemaVersion       int                   `json:"schema_version"`
	MediaType           string                `json:"mediaType"`
	ArtifactType        string                `json:"artifactType,omitempty"`
	Manifests           []Manifest            `json:"manifests"`
	Subject             *imgspecv1.Descriptor `json:"subject,omitempty"`
	Annotations         map[string]string     `json:"annotations,omitempty"`
	NewUnspecifiedField []byte                `json:"newUnspecifiedField"`
}

type OCIManifestList struct {
	SchemaVersion int                               `json:"schemaVersion"`
	MediaType     string                            `json:"mediaType"`
	ArtifactType  string                            `json:"artifactType,omitempty"`
	Manifests     []manifestlist.ManifestDescriptor `json:"manifests"`
	Subject       *imgspecv1.Descriptor             `json:"subject,omitempty"`
	Annotations   map[string]string                 `json:"annotations,omitempty"`
}

func (ml Index) ToOCIDescriptor() (md imgspecv1.Descriptor) {
	md = imgspecv1.Descriptor{
		MediaType:   ml.MediaType,
		Digest:      digest.Digest(ml.Digest),
		Size:        int64(ml.Size),
		Annotations: ml.Annotations,
	}
	if ml.ArtifactType != "" {
		md.ArtifactType = ml.ArtifactType
	}
	return
}

func (ml Index) ToOCIBytes() (b []byte) {
	oci := OCIManifestList{
		SchemaVersion: ml.SchemaVersion,
		MediaType:     ml.MediaType,
		Subject:       ml.Subject,
		Annotations:   ml.Annotations,
	}
	if ml.ArtifactType != "" {
		oci.ArtifactType = ml.ArtifactType
	}
	// Sort by digest
	digList := []string{}
	digMap := map[string]Manifest{}
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

func (ml Index) WriteWWWHeader(w http.ResponseWriter) {
	// TODO: Not sure if that is stable over time. We might need to
	//
	b := ml.ToOCIBytes()
	size := len(b)
	ml.Digest = digest.FromBytes(b).String()
	w.Header().Set(ContentType, ml.MediaType)
	w.Header().Set(ContentLength, fmt.Sprint(size))
	w.Header().Set(DockerContentDigest, ml.Digest)
	w.Header().Set("Etag", ml.Digest)

}

func (ml Index) WriteWWW(w http.ResponseWriter) {
	b := ml.ToOCIBytes()
	size := len(b)
	ml.Digest = digest.FromBytes(b).String()
	w.Header().Set(ContentType, ml.MediaType)
	w.Header().Set(ContentLength, fmt.Sprint(size))
	w.Header().Set(DockerContentDigest, ml.Digest)
	w.Header().Set("Etag", ml.Digest)
	w.Write(b)
}

func NewManifestListFromData(b []byte) (ml Index, err error) {
	err = json.Unmarshal(b, &ml)
	if err != nil {
		return
	}
	ml.Digest = digest.FromBytes(b).String()
	return
}

func (ml Index) String() (out string) {
	b, err := json.MarshalIndent(ml, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error marshaling Index: %v", err)
	}
	out = string(b)
	return out
}
