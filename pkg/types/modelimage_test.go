package types

import (
	"encoding/json"
	"testing"

	"github.com/opencontainers/go-digest"
)

func TestAssembleModelImage_OrderAndInvariant(t *testing.T) {
	layers := []ModelLayerDescriptor{
		{Digest: digest.FromString("shard1"), Size: 111},
		{Digest: digest.FromString("shard2"), Size: 222},
		{Digest: digest.FromString("shard3"), Size: 333},
	}
	cfg, cfgDig, mani, maniDig, err := AssembleModelImage("", "", layers, "", "", 0, nil)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}

	// config digest must be over the exact config bytes
	if cfgDig != digest.FromBytes(cfg) {
		t.Fatal("config digest mismatch")
	}
	if maniDig != digest.FromBytes(mani) {
		t.Fatal("manifest digest mismatch")
	}

	var m ociManifest
	if err := json.Unmarshal(mani, &m); err != nil {
		t.Fatal(err)
	}
	if m.SchemaVersion != 2 || m.MediaType != ociManifestMediaType {
		t.Fatalf("bad manifest envelope: %+v", m)
	}
	if m.Config.Digest != cfgDig.String() || m.Config.Size != int64(len(cfg)) {
		t.Fatalf("config desc mismatch: %+v", m.Config)
	}
	if len(m.Layers) != 3 {
		t.Fatalf("want 3 layers, got %d", len(m.Layers))
	}
	for i, l := range m.Layers {
		if l.MediaType != ModelLayerMediaType {
			t.Fatalf("layer %d mediaType=%s", i, l.MediaType)
		}
		if l.Digest != layers[i].Digest.String() || l.Size != layers[i].Size {
			t.Fatalf("layer %d order/desc mismatch: %+v", i, l)
		}
	}

	// config RootFS.DiffIDs order == layer order == input order
	var c ociImageConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		t.Fatal(err)
	}
	if len(c.RootFS.DiffIDs) != 3 {
		t.Fatalf("want 3 diff_ids, got %d", len(c.RootFS.DiffIDs))
	}
	for i := range layers {
		if c.RootFS.DiffIDs[i] != layers[i].Digest.String() {
			t.Fatalf("diff_id %d order mismatch", i)
		}
	}
	if c.Architecture != "amd64" || c.OS != "linux" {
		t.Fatalf("defaults not applied: %+v", c)
	}
}

func TestAssembleModelImage_Deterministic(t *testing.T) {
	layers := []ModelLayerDescriptor{{Digest: digest.FromString("a"), Size: 1}}
	c1, cd1, m1, md1, _ := AssembleModelImage("amd64", "linux", layers, "", "", 0, nil)
	c2, cd2, m2, md2, _ := AssembleModelImage("amd64", "linux", layers, "", "", 0, nil)
	if string(c1) != string(c2) || cd1 != cd2 || string(m1) != string(m2) || md1 != md2 {
		t.Fatal("assembly not deterministic")
	}
}

func TestAssembleModelImage_Empty(t *testing.T) {
	if _, _, _, _, err := AssembleModelImage("", "", nil, "", "", 0, nil); err == nil {
		t.Fatal("expected error for no layers")
	}
}

func TestAssembleModelImage_Subject(t *testing.T) {
	layers := []ModelLayerDescriptor{{Digest: digest.FromString("a"), Size: 1}}
	subDig := digest.FromString("artifact").String()
	subMT := "application/vnd.qnib.model.artifact"

	// With a subject: the manifest carries the OCI subject descriptor.
	_, _, mani, _, err := AssembleModelImage("", "", layers, subDig, subMT, 42, nil)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	var m ociManifest
	if err := json.Unmarshal(mani, &m); err != nil {
		t.Fatal(err)
	}
	if m.Subject == nil {
		t.Fatal("expected subject to be set")
	}
	if m.Subject.Digest != subDig || m.Subject.MediaType != subMT || m.Subject.Size != 42 {
		t.Fatalf("subject desc mismatch: %+v", m.Subject)
	}

	// Without a subject: the field is omitted.
	_, _, mani2, _, _ := AssembleModelImage("", "", layers, "", "", 0, nil)
	var m2 ociManifest
	if err := json.Unmarshal(mani2, &m2); err != nil {
		t.Fatal(err)
	}
	if m2.Subject != nil {
		t.Fatalf("expected no subject, got %+v", m2.Subject)
	}
}

// TestAssembleModelImage_Annotations verifies a config artefact carries its
// composition annotations ({profile, runtime, quant}) on the manifest, and that
// an empty annotation map is omitted.
func TestAssembleModelImage_Annotations(t *testing.T) {
	layers := []ModelLayerDescriptor{{Digest: digest.FromString("a"), Size: 1}}
	ann := ConfigCompositionAnnotations("vram#12gb", "q8-kxl", "llama-server")
	_, _, mani, _, err := AssembleModelImage("", "", layers, "", "", 0, ann)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	var m ociManifest
	if err := json.Unmarshal(mani, &m); err != nil {
		t.Fatal(err)
	}
	if m.Annotations[ModelProfileAnnotation] != "vram#12gb" ||
		m.Annotations[ModelQuantAnnotation] != "q8-kxl" ||
		m.Annotations[ModelRuntimeAnnotation] != "llama-server" {
		t.Fatalf("annotations mismatch: %+v", m.Annotations)
	}

	// Empty annotations → field omitted.
	_, _, mani2, _, _ := AssembleModelImage("", "", layers, "", "", 0, nil)
	var m2 ociManifest
	if err := json.Unmarshal(mani2, &m2); err != nil {
		t.Fatal(err)
	}
	if m2.Annotations != nil {
		t.Fatalf("expected no annotations, got %+v", m2.Annotations)
	}
}
