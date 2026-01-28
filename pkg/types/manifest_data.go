package types

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/minio/minio-go/v7/pkg/set"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"

	"github.com/sirupsen/logrus"
)

var (
	containerConfigMediaTypes = set.CreateStringSet(
		imgspecv1.MediaTypeImageConfig,
		"application/vnd.docker.container.image.v1+json",
	)
)

type ManifestData struct {
	ID           int64         `json:"-"`
	Type         string        `json:"-"`
	Origin       string        `json:"-"`
	Digest       string        `json:"digest"`
	Data         []byte        `json:"data,omitempty"`
	MediaType    string        `json:"mediaType"`
	ArtifactType string        `json:"artifactType,omitempty"`
	Size         int64         `json:"size,omitempty"`
	Platform     Platform      `json:"platform,omitempty"`
	Upstream     Upstream      `json:"upstream,omitempty"`
	Config       Config        `json:"config,omitempty"`
	Layers       []Layer       `json:"layers,omitempty"`
	Label        string        `json:"label,omitempty"`
	Profile      string        `json:"profile,omitempty"`
	Annotations  Annotations   `json:"annotations,omitempty"`
	Subject      *ManifestData `json:"subject,omitempty"`
}

func (m ManifestData) ToBytes() (data []byte, err error) {
	data, err = json.Marshal(m)
	return
}

func (m ManifestData) ToOCIManifestDescriptor() (md manifestlist.ManifestDescriptor) {
	feats := []string{}
	logrus.Warnf("Platofmr: %#v", m.Platform)
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

func ManifestDataFromOciDescriptor(md v1.Descriptor) (m ManifestData) {
	a, err := NewAnnotationsFromMap(md.Annotations)
	if err != nil {
		logrus.Errorf("while creating Annotations from map: %v", err)
	}
	m = ManifestData{
		Digest:      md.Digest.String(),
		MediaType:   md.MediaType,
		Size:        md.Size,
		Annotations: a,
	}
	if md.Platform != nil {
		m.Platform = Platform{
			Arch:    md.Platform.Architecture,
			OS:      md.Platform.OS,
			Variant: md.Platform.Variant,
		}
	}
	return
}

func (md ManifestData) ToOCIDescriptor() (mdes imgspecv1.Descriptor) {
	mdes = imgspecv1.Descriptor{
		Digest:      digest.Digest(md.Digest),
		MediaType:   md.MediaType,
		Size:        md.Size,
		Annotations: md.Annotations.ToMap(),
	}
	if md.Platform.Arch != "" || md.Platform.OS != "" || md.Platform.Variant != "" {
		mdes.Platform = &imgspecv1.Platform{
			Architecture: md.Platform.Arch,
			OS:           md.Platform.OS,
			Variant:      md.Platform.Variant,
		}
	}
	return mdes
}

func (m ManifestData) ToManifestDescriptor() (md ManifestDescriptor) {
	feats := []string{}
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
	Layers      []distribution.Descriptor `json:"layers"`
	Subject     *imgspecv1.Descriptor     `json:"subject,omitempty"`
	Annotations map[string]string         `json:"annotations,omitempty"`
}

func (om OCIManifest) ToBytes() (b []byte, err error) {
	b, err = json.Marshal(om)
	err = errors.Wrap(err, "while marshal OCIManifest")
	return
}

func (m ManifestData) ToOCIBytes() (b []byte) {
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

func (m ManifestData) WriteWWWHead(w http.ResponseWriter) (err error) {
	if m.MediaType == "" {
		return fmt.Errorf("Manifest.MediaType is empty")
	}
	dataLen := len(m.Data)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", dataLen))
	w.Header().Set("Content-Type", m.MediaType)
	w.Header().Set("Docker-Content-Digest", m.Digest)
	w.Header().Set("Etag", m.Digest)
	if m.Subject != nil {
		w.Header().Set("OCI-Subject", m.Subject.Digest)
	}
	return
}
func (m ManifestData) WriteWWWHeader(w http.ResponseWriter) (err error) {
	if m.MediaType == "" {
		return fmt.Errorf("Manifest.MediaType is empty")
	}
	//w.Header().Set("Location", fmt.Sprintf("%s/v2/%s/manifests/%s", m.Upstream.Reg, m.Upstream.Repo, m.Upstream.Tag))
	//dataLen := len(m.Data)
	dataLen := 0
	w.Header().Set("Content-Length", fmt.Sprintf("%d", dataLen))
	w.Header().Set("Content-Type", m.MediaType)
	w.Header().Set("Docker-Content-Digest", m.Digest)
	w.Header().Set("Etag", m.Digest)
	if m.Subject != nil {
		w.Header().Set("OCI-Subject", m.Subject.Digest)
	}
	return
}

func (m ManifestData) WriteWWW(w http.ResponseWriter) {
	size := len(m.Data)
	w.Header().Set("Content-Length", fmt.Sprint(size))
	b := m.Data
	dig := digest.FromBytes(b)
	if dig.String() != m.Digest {
		logrus.Error("Digest mismatch for ManifestData")
	}
	w.Header().Set("Content-Type", m.MediaType)
	w.Header().Set("Docker-Content-Digest", m.Digest)
	w.Header().Set("Etag", m.Digest)
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

func (m *OCIManifest) IsArtifact() (isArtifact bool) {
	if !containerConfigMediaTypes.Contains(m.Config.MediaType) {
		isArtifact = true
		return
	}
	return false
}
