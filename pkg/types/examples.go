package types

import (
	"embed"

	"github.com/docker/distribution"
	"github.com/opencontainers/go-digest"
)

//go:embed data/*
var f embed.FS

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

func FT_amd_ConfigByte() []byte {
	// read file and return bytes
	dat, err := f.ReadFile("data/cfg/83fdbde95f56beb69853240df571a7f085c561b4f8d914c5cc5e0f171dcb86df.dat")
	if err != nil {
		panic(err)
	}
	return dat
}

func FT_arm_Config() Config {
	cfg, _ := NewConfigFromBytes(FT_arm_ConfigByte())
	return cfg
}

func FT_amd_Config() Config {
	cfg, _ := NewConfigFromBytes(FT_amd_ConfigByte())
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

func FT_amd_Upstream() Upstream {
	return Upstream{
		Reg:    "registry.gitlab.com",
		Repo:   "qnib-pub-containers/qnib/plain-featuretest",
		Tag:    "20220715-amd64-generic",
		Digest: digest.Digest("sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5"),
	}
}

func FT_arm_Upstream() Upstream {
	return Upstream{
		Reg:    "registry.gitlab.com",
		Repo:   "qnib-pub-containers/qnib/plain-featuretest",
		Tag:    "20220715-arm64-generic",
		Digest: digest.Digest("sha256:5bf8eee20fde14eb6e3e924c18fafa4d816e131f315810875136f0438ff7beed"),
	}
}
func CollectionTag_arm() CollectionTag {
	return CollectionTag{Tag: "arm64"}
}

func CollectionTag_Latest() CollectionTag {
	return CollectionTag{Tag: "latest"}
}

func CollectionTag_Generic() CollectionTag {
	return CollectionTag{Tag: "generic"}
}

func FT_generic_Collection() Collection {
	return Collection{
		Repo: "qnib/featuretest",
		Tag:  CollectionTag_Generic(),
		ManifestLists: []ManifestList{
			FT_arm_Manifestlist(),
			FT_amd_Manifestlist(),
		},
	}
}

func FT_arm_Collection() Collection {
	return Collection{
		Repo: "qnib/featuretest",
		Tag:  CollectionTag_Latest(),
		ManifestLists: []ManifestList{
			FT_arm_Manifestlist(),
		},
	}
}

func FT_amd_Layer0Reference() distribution.Descriptor {
	return distribution.Descriptor{
		MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
		Size:      2796496,
		Digest:    "sha256:88ecf269dec31566a8e6b05147732fe34d32bc608de0d636dffaba659230a515",
	}
}

func FT_amd_Layer1Reference() distribution.Descriptor {
	return distribution.Descriptor{
		MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
		Size:      304,
		Digest:    "sha256:a2f2473b6a7e9b3d17c9873e41d04808b4c0310e0d4f7c13e03fa3f9a3a14bcc",
	}
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
