package types

import (
	"embed"

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

func FT_amd64_ConfigByte() []byte {
	// read file and return bytes
	dat, err := f.ReadFile("data/cfg/83fdbde95f56beb69853240df571a7f085c561b4f8d914c5cc5e0f171dcb86df.dat")
	if err != nil {
		panic(err)
	}
	return dat
}

func FT_amd64_Config() Config {
	cfg, _ := NewConfigFromBytes(FT_amd64_ConfigByte())
	return cfg
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

func FT_amd_Upstream() Upstream {
	return Upstream{
		Reg:    "registry.gitlab.com",
		Repo:   "qnib-pub-containers/qnib/plain-featuretest",
		Tag:    "20220715-amd64-generic",
		Digest: digest.Digest("sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5"),
	}
}
func FT_amd64_Upstream() Upstream {
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

func CollectionTag_amd64() CollectionTag {
	return CollectionTag{Tag: "amd64"}
}

func CollectionTag_arm() CollectionTag {
	return CollectionTag{Tag: "arm64"}
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
		Tag:  CollectionTag_arm(),
		ManifestLists: []ManifestList{
			FT_arm_Manifestlist(),
		},
	}
}

func FT_amd64_Collection() Collection {
	return Collection{
		Repo: "qnib/featuretest",
		Tag:  CollectionTag_amd64(),
		ManifestLists: []ManifestList{
			FT_amd_Manifestlist(),
		},
	}
}
