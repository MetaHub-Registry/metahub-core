package types

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/sirupsen/logrus"
)

type Manifest struct {
	ID        int64    `json:"-"`
	Digest    string   `json:"digest"`
	Data      []byte   `json:"data,omitempty"`
	MediaType string   `json:"media_type"`
	Size      int64    `json:"size"`
	Platform  Platform `json:"platform"`
	Upstream  Upstream `json:"upstream"`
	Config    Config   `json:"config"`
	Layers    []Layer  `json:"layers"`
	Profile   Profile  `json:"profile"`
	//Annotations Annotations `json:"annotations"`
}

func (m Manifest) ToBytes() (data []byte, err error) {
	data, err = json.Marshal(m)
	return
}

func (m Manifest) ToOCIManifestDescriptor() (md manifestlist.ManifestDescriptor) {
	feats := []string{}
	for _, f := range strings.Split(m.Platform.Features, ",") {
		if f != "" {
			feats = append(feats, f)
		}
	}
	strings.Split(m.Platform.Features, ",")
	return manifestlist.ManifestDescriptor{
		distribution.Descriptor{
			Digest:    digest.Digest(m.Digest),
			MediaType: m.MediaType,
			Size:      m.Size,
		},
		manifestlist.PlatformSpec{
			Architecture: m.Platform.Arch,
			OS:           m.Platform.OS,
			Variant:      m.Platform.Variant,
			Features:     feats,
		},
	}
}

func (m Manifest) ToManifestDescriptor() (md ManifestDescriptor) {
	feats := []string{}
	for _, f := range strings.Split(m.Platform.Features, ",") {
		if f != "" {
			feats = append(feats, f)
		}
	}
	strings.Split(m.Platform.Features, ",")
	return ManifestDescriptor{
		Digest:    m.Digest,
		MediaType: m.MediaType,
		Size:      int(m.Size),
		Platform: manifestlist.PlatformSpec{
			Architecture: m.Platform.Arch,
			OS:           m.Platform.OS,
			Variant:      m.Platform.Variant,
			Features:     feats,
		},
	}
}

type OCIManifest struct {
	// SchemaVersion is the image manifest schema that this image follows
	SchemaVersion int `json:"schemaVersion"`
	// MediaType is the media type of this schema.
	MediaType string `json:"mediaType,omitempty"`
	// Config references the image configuration as a blob.
	Config imgspecv1.Descriptor `json:"config"`
	// Layers lists descriptors for the layers referenced by the
	// configuration.
	Layers []distribution.Descriptor `json:"layers"`
}

func (m Manifest) ToOCIBytes() (b []byte) {
	oci := OCIManifest{
		SchemaVersion: 2,
		MediaType:     m.MediaType,
		Config:        m.Config.ToDescriptor(),
		Layers:        []distribution.Descriptor{},
	}
	for _, l := range m.Layers {
		oci.Layers = append(oci.Layers, l.ToDescriptor())
	}
	b, _ = json.Marshal(oci)
	return
}

func (m Manifest) WriteWWWHeader(w http.ResponseWriter) {
	if m.MediaType == "" {
		logrus.Fatal("Manifest.MediaType is empty")
	}
	//w.Header().Set("Location", fmt.Sprintf("%s/v2/%s/manifests/%s", m.Upstream.Reg, m.Upstream.Repo, m.Upstream.Tag))
	w.Header().Set("Content-Length", "0")
	w.Header().Set("Content-Type", m.MediaType)
	w.Header().Set("Docker-Content-Digest", m.Digest)
	w.Header().Set("Etag", m.Digest)

}

func (m Manifest) WriteWWWBody(w http.ResponseWriter) {
	size := len(m.Data)
	w.Header().Set("Content-Length", fmt.Sprint(size))
	b := m.Data
	dig := digest.FromBytes(b)
	logrus.Warnf("DIGEST.FromBytes: %s", dig.String())
	logrus.Warnf("DIGEST.FromMani: %s", m.Digest)
	w.Write(b)
}

/*
func (m *Manifest) UpdateAnnotations(anno Annotations) (err error) {
	for k, v := range anno.Map {
		err = m.Annotations.AddAnnotation(k, v)
		if err != nil {
			return
		}
	}
	return
}
*/
