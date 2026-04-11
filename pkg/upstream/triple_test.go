package upstream

import (
	"testing"

	"github.com/metahub-registry/metahub-core/pkg/types"
)

func TestNewUpstreamTriple(t *testing.T) {
	up := types.Upstream{
		Digest:    "sha256:6ec0f660682c2f5b391af1d81d3b4839da06f050269546177b28e4b51d8d4c30",
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Size:      3456,
		Reg:       "myregistry.io",
		Repo:      "myrepo/myimage",
		Tag:       "latest",
	}
	upt, err := NewUpstreamTripleBase(up.Reg, up.Repo, up.Tag, up.MediaType, string(up.Digest), up.Size)
	if err != nil {
		t.Fatal(err)
	}
	if upt.Digest.String() != string(up.Digest) {
		t.Fatalf("expected digest %s, got %s", up.Digest, upt.Digest.String())
	}
	if upt.MediaType != up.MediaType {
		t.Fatalf("expected media type %s, got %s", up.MediaType, upt.MediaType)
	}
	if upt.Size != up.Size {
		t.Fatalf("expected size %d, got %d", up.Size, upt.Size)
	}
	if upt.Reg != up.Reg {
		t.Fatalf("expected reg %s, got %s", up.Reg, upt.Reg)
	}
	if upt.Repo != up.Repo {
		t.Fatalf("expected repo %s, got %s", up.Repo, upt.Repo)
	}
	if upt.Tag != up.Tag {
		t.Fatalf("expected tag %s, got %s", up.Tag, upt.Tag)
	}
}
func TestUpstreamTripleGetLabel(t *testing.T) {
	up := UpstreamTriple{
		Digest:    "sha256:1234567890abcdef",
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Size:      3456,
		Reg:       "myregistry.io",
		Repo:      "myrepo/myimage",
		Tag:       "latest",
	}
	expected := "myregistry.io/myrepo/myimage:latest"
	s := up.GetLabel()
	if s != expected {
		t.Fatalf("expected %s, got %s", expected, s)
	}
	s = up.GetObjPath()
	expected = "myregistry.io/myrepo/myimage/latest"
	if s != expected {
		t.Fatalf("expected %s, got %s", expected, s)
	}
}

func TestUpstreamToUpstreamTriple(t *testing.T) {
	up := types.Upstream{
		Digest: "sha256:1234567890abcdef",
		Reg:    "myregistry.io",
		Repo:   "myrepo/myimage",
		Tag:    "latest",
	}
	upt, err := UpstreamToUpstreamTriple(up, "application/vnd.oci.image.manifest.v1+json")
	if err != nil {
		t.Fatal(err)
	}
	if upt.Digest.String() != string(up.Digest) {
		t.Fatalf("expected digest %s, got %s", up.Digest, upt.Digest.String())
	}
	if upt.Reg != up.Reg {
		t.Fatalf("expected reg %s, got %s", up.Reg, upt.Reg)
	}
	if upt.Repo != up.Repo {
		t.Fatalf("expected repo %s, got %s", up.Repo, upt.Repo)
	}
	if upt.Tag != up.Tag {
		t.Fatalf("expected tag %s, got %s", up.Tag, upt.Tag)
	}
}

func TestNewUpstreamTripleFromMap(t *testing.T) {
	m := map[string]any{
		"reg":       "myregistry.io",
		"repo":      "myrepo/myimage",
		"tag":       "latest",
		"mediaType": "application/vnd.oci.image.manifest.v1+json",
		"digest":    "sha256:6ec0f660682c2f5b391af1d81d3b4839da06f050269546177b28e4b51d8d4c30",
		"size":      int64(3456),
	}
	upt, err := NewUpstreamTriple(m)
	if err != nil {
		t.Fatal(err)
	}
	if upt.Repo != "myrepo/myimage" {
		t.Fatalf("expected myrepo/myimage, got %s", upt.Repo)
	}
}

func TestNewUpstreamTripleBase_InvalidDigest(t *testing.T) {
	_, err := NewUpstreamTripleBase("reg", "repo", "tag", "mt", "invalid", 0)
	if err == nil {
		t.Fatal("expected error for invalid digest")
	}
}

func TestUpstreamTripleToBytes(t *testing.T) {
	upt := UpstreamTriple{Reg: "reg", Repo: "repo", Tag: "tag"}
	b := upt.ToBytes()
	if len(b) == 0 {
		t.Fatal("expected non-empty bytes")
	}
}
