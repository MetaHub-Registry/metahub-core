package types

import (
	"encoding/json"
	"fmt"

	"github.com/opencontainers/go-digest"
)

// ociConfigMediaType is defined in config.go (reused here).
const ociManifestMediaType = "application/vnd.oci.image.manifest.v1+json"

// ModelLayerDescriptor identifies one synthesised model layer. For an
// uncompressed tar the layer Digest equals the DiffID, so both the manifest
// descriptor and the config RootFS reuse Digest.
type ModelLayerDescriptor struct {
	Digest digest.Digest
	Size   int64
}

type ociDescriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

type ociRootFS struct {
	Type    string   `json:"type"`
	DiffIDs []string `json:"diff_ids"`
}

type ociImageConfig struct {
	Architecture string    `json:"architecture"`
	OS           string    `json:"os"`
	RootFS       ociRootFS `json:"rootfs"`
}

type ociManifest struct {
	SchemaVersion int             `json:"schemaVersion"`
	MediaType     string          `json:"mediaType"`
	Config        ociDescriptor   `json:"config"`
	Layers        []ociDescriptor `json:"layers"`
	// Subject is the optional OCI subject reference (referrers API): the source
	// ORAS model artifact a model-layer image was repackaged from, or — for a
	// config image (Decision A) — the model-layer image the config composes with.
	Subject *ociDescriptor `json:"subject,omitempty"`
	// Annotations carry composition metadata on a config artefact
	// ({profile, runtime, quant}); omitted when empty.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// AssembleModelImage builds a minimal OCI image (config + manifest) whose only
// content is the given model layers, in order. The config RootFS.DiffIDs and the
// manifest layer descriptors follow the same order (the ORAS blob order, per the
// design). Everything is deterministic: struct-ordered JSON with no timestamps,
// so the same layers always yield the same config/manifest digests.
//
// Returns the config bytes + digest and the manifest bytes + digest, ready to
// PutBlob / PutManifest via the metalib MinIO backend.
//
// When subjectDigest is non-empty, the manifest carries an OCI `subject`
// reference to that descriptor (the source ORAS model artifact), making the
// model-layer image a referrer of the artifact.
func AssembleModelImage(arch, osName string, layers []ModelLayerDescriptor, subjectDigest, subjectMediaType string, subjectSize int64, annotations map[string]string) (cfg []byte, cfgDig digest.Digest, mani []byte, maniDig digest.Digest, err error) {
	if len(layers) == 0 {
		err = fmt.Errorf("AssembleModelImage: no layers")
		return
	}
	if arch == "" {
		arch = "amd64"
	}
	if osName == "" {
		osName = "linux"
	}

	diffIDs := make([]string, len(layers))
	descs := make([]ociDescriptor, len(layers))
	for i, l := range layers {
		if l.Digest == "" {
			err = fmt.Errorf("AssembleModelImage: layer %d has empty digest", i)
			return
		}
		diffIDs[i] = l.Digest.String()
		descs[i] = ociDescriptor{MediaType: ModelLayerMediaType, Digest: l.Digest.String(), Size: l.Size}
	}

	cfg, err = json.Marshal(ociImageConfig{
		Architecture: arch, OS: osName,
		RootFS: ociRootFS{Type: "layers", DiffIDs: diffIDs},
	})
	if err != nil {
		return
	}
	cfgDig = digest.FromBytes(cfg)

	m := ociManifest{
		SchemaVersion: 2,
		MediaType:     ociManifestMediaType,
		Config:        ociDescriptor{MediaType: ociConfigMediaType, Digest: cfgDig.String(), Size: int64(len(cfg))},
		Layers:        descs,
	}
	if subjectDigest != "" {
		m.Subject = &ociDescriptor{MediaType: subjectMediaType, Digest: subjectDigest, Size: subjectSize}
	}
	if len(annotations) > 0 {
		m.Annotations = annotations
	}
	mani, err = json.Marshal(m)
	if err != nil {
		return
	}
	maniDig = digest.FromBytes(mani)
	return
}
