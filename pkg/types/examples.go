package types

import (
	"embed"

	"github.com/docker/distribution"
	"github.com/opencontainers/go-digest"
)

//go:embed data/*
var f embed.FS

func ManifestReferenceByte_FT_arm() []byte {
	return []byte(`{
		"reg": "registry.gitlab.com",
		"repo": "qnib-pub-containers/qnib/plain-featuretest",
		"tag": "20220715-arm64-generic",
		"mediaType": "",
		"size": 735,
		"digest": "sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5",
		"platform": {
		  "architecture": "amd64",
		  "os": "linux"
		}
	  }`)
}

func Example_Platform_amd64() Platform {
	return Platform{
		Arch:     "amd64",
		OS:       "linux",
		Variant:  "",
		Features: "",
	}
}

func Example_Platform_broadwell() Platform {
	return Platform{
		Arch:     "amd64",
		OS:       "linux",
		Variant:  "",
		Features: "cpu:broadwell",
	}
}

func FT_arm_ConfigByte() []byte {
	// read file and return bytes
	dat, err := f.ReadFile("data/cfg/cf5c26263929e84d7373d23b2692222cceceb3175b3325b183ead2ab6b5a5ce3.dat")
	if err != nil {
		panic(err)
	}
	return dat
}

func FT_arm_Config() Config {
	cfg, _ := NewConfigFromBytes(FT_arm_ConfigByte())
	return cfg
}

func FT_arm_Upstream_Generic() Upstream {
	return Upstream{
		Reg:    "registry.gitlab.com",
		Repo:   "qnib-pub-containers/qnib/plain-featuretest",
		Tag:    "20220715-arm64-generic",
		Digest: digest.Digest("sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5"),
	}
}

func FT_arm_Upstream() Upstream {
	return Upstream{
		Reg:    "registry.gitlab.com",
		Repo:   "qnib-pub-containers/qnib/plain-featuretest",
		Tag:    "20220715-arm64-generic",
		Digest: digest.Digest("sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5"),
	}
}

func CollectionTag_Latest() CollectionTag {
	return CollectionTag{Tag: "latest"}
}

func FT_arm_Collection() Collection {
	return Collection{
		Repo: "qnib/featuretest",
		Tag:  CollectionTag_Latest(),
		ManifestLists: []ManifestList{
			FT_generic_arm_Manifestlist(),
		},
	}
}

func FT_generic_arm_Manifestlist() ManifestList {
	dat, err := f.ReadFile("data/manifestlists/d46f2503b600eac6066ba46c034425096087875de9480334b3471347d14bf227.dat")
	if err != nil {
		panic(err)
	}
	dig := digest.FromBytes(dat).String()
	if dig != "sha256:d46f2503b600eac6066ba46c034425096087875de9480334b3471347d14bf227" {
		panic("digest mismatch")
	}
	return ManifestList{
		SchemaVersion: 2,
		Digest:        dig,
		MediaType:     "application/vnd.docker.distribution.manifest.list.v2+json",
		Manifests: []Manifest{
			FT_arm_Manifest(),
		},
	}
}

func FT_arm_Manifest() Manifest {
	dat, err := f.ReadFile("data/manifests/ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5.dat")
	if err != nil {
		panic(err)
	}
	cfg := FT_arm_Config()
	l0 := FT_arm_Layer0Reference()
	l1 := FT_arm_Layer1Reference()
	up := FT_arm_Upstream()
	mani := Manifest{
		Digest:    string(digest.FromBytes(dat)),
		Size:      735,
		MediaType: "application/vnd.docker.distribution.manifest.v2+json",
		Data:      dat,
		Config:    cfg,
		Upstream:  up,
		Platform: Platform{
			Arch:    cfg.Cfg.Architecture,
			OS:      cfg.Cfg.OS,
			Variant: cfg.Cfg.Variant,
		},

		Layers: []Layer{
			{Digest: l0.Digest.String(), Size: l0.Size, MediaType: l0.MediaType, Data: FT_arm_Layer0Byte()},
			{Digest: l1.Digest.String(), Size: l1.Size, MediaType: l1.MediaType, Data: FT_arm_Layer1Byte()},
		},
	}
	return mani
}

// Layer
func FT_arm_Layer0Reference() distribution.Descriptor {
	return distribution.Descriptor{
		MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
		Size:      2693535,
		Digest:    "sha256:6ec0f660682c2f5b391af1d81d3b4839da06f050269546177b28e4b51d8d4c30",
	}
}

func FT_arm_Layer1Reference() distribution.Descriptor {
	return distribution.Descriptor{
		MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
		Size:      304,
		Digest:    "sha256:297a529dab3ab8530adef75e34219e97ec0f3cb2fd39efdb13a1cc0a70ece15a",
	}
}

func FT_arm_Layer0Byte() []byte {
	// read file and return bytes
	dat, err := f.ReadFile("data/layer/6ec0f660682c2f5b391af1d81d3b4839da06f050269546177b28e4b51d8d4c30.dat")
	if err != nil {
		panic(err)
	}
	return dat
}

func FT_arm_Layer1Byte() []byte {
	// read file and return bytes
	dat, err := f.ReadFile("data/layer/297a529dab3ab8530adef75e34219e97ec0f3cb2fd39efdb13a1cc0a70ece15a.dat")
	if err != nil {
		panic(err)
	}
	return dat
}

func FT_arm_BlobUpload_Layer0() (OCIBlobUpload, []byte) {
	r := FT_arm_Layer0Reference()
	bu := OCIBlobUpload{
		UploadedBy: "test",
		BlobType:   "layer",
		BlobID:     1,
		Repo:       "qnib/featuretest",
		Digest:     r.Digest.String(),
	}
	return bu, FT_arm_Layer0Byte()
}

func FT_arm_BlobUpload_Config() (OCIBlobUpload, []byte) {
	cfg := FT_arm_Config()
	bu := OCIBlobUpload{
		UploadedBy: "test",
		BlobType:   "config",
		BlobID:     1,
		Repo:       "qnib/featuretest",
		Digest:     cfg.Digest,
	}
	return bu, FT_arm_ConfigByte()
}
