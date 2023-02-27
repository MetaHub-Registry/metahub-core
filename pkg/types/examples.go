package types

import (
	"embed"

	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

//go:embed data/*
var f embed.FS

var (
	/*Example_PlatformAMD64 = []Platform{
		{Arch: "amd64", Variant: "", OS: "linux", Features: "cpu:broadwell"},
		{Arch: "amd64", Variant: "", OS: "linux", Features: "cpu:cascadelake"},
		{Arch: "amd64", Variant: "", OS: "linux", Features: "cpu:skylake"},
		{Arch: "amd64", Variant: "", OS: "linux", Features: "cpu:skylake_avx512"},
		{Arch: "amd64", Variant: "", OS: "linux", Features: "cpu:icelake"},
		{Arch: "amd64", Variant: "", OS: "linux", Features: "cpu:zen"},
		{Arch: "amd64", Variant: "", OS: "linux", Features: "cpu:zen2"},
		{Arch: "amd64", Variant: "", OS: "linux", Features: "cpu:zen3"},
		{Arch: "amd64", Variant: "", OS: "linux", Features: "cpu:zen4"},
	}
	Example_PlatformARM64 = []Platform{
		{Arch: "arm64", Variant: "v8", OS: "linux", Features: "cpu:a64fx"},
		{Arch: "arm64", Variant: "v8", OS: "linux", Features: "cpu:graviton3"},
		{Arch: "arm64", Variant: "v8", OS: "linux", Features: "cpu:graviton3e"},
	}
	*/
	Example_PlatformGeneric = []Platform{
		{Arch: "amd64", Variant: "", OS: "linux"},
		{Arch: "arm", Variant: "v6", OS: "linux"},
		{Arch: "arm", Variant: "v7", OS: "linux"},
		{Arch: "arm64", Variant: "", OS: "linux"},
		{Arch: "arm64", Variant: "v8", OS: "linux"},
		{Arch: "riscv64", Variant: "", OS: "linux"},
	}
)

func Example_Platform_amd64() Platform {
	return Platform{
		Arch:    "amd64",
		OS:      "linux",
		Variant: "",
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
	dat, err := f.ReadFile("data/cfg/fba3e3993bfe856a766b683c73e1ddeb03f2d359d7bd7700095682d94c3fcebf.dat")
	if err != nil {
		panic(err)
	}
	return dat
}

func FT_amd64_b109_ConfigByte() []byte {
	// read file and return bytes
	dat, err := f.ReadFile("data/cfg/b109e1dac65af00234a8a7f817dcb1e18bc18ca97a5b805283995ebf1137e471.dat")
	if err != nil {
		panic(err)
	}
	return dat
}

func FT_amd64_Config() Config {
	cfg, _ := NewConfigFromBytes(FT_amd64_ConfigByte())
	return cfg
}

func FT_amd64_OCIManifest() OCIManifest {
	cfg := FT_amd64_Config()
	return OCIManifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.manifest.v1+json",
		Config: imgspecv1.Descriptor{
			MediaType: cfg.MediaType,
			Digest:    digest.Digest(cfg.Digest),
			Size:      cfg.Size,
			Data:      cfg.Data,
			Platform: &imgspecv1.Platform{
				Architecture: cfg.Cfg.Architecture,
				OS:           cfg.Cfg.OS,
				OSVersion:    cfg.Cfg.OSVersion,
				OSFeatures:   cfg.Cfg.OSFeatures,
				Variant:      cfg.Cfg.Variant,
			},
		},
	}
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
	ct := CollectionTag_Generic()
	return Collection{
		Repo:   "qnib/featuretest",
		Tag:    ct.Tag,
		ColTag: ct,
		ManifestLists: []ManifestList{
			FT_arm_Manifestlist(),
			FT_amd_Manifestlist(),
		},
	}
}

func FT_arm_Collection() Collection {
	ct := CollectionTag_arm()
	return Collection{
		Repo:   "qnib/featuretest",
		Tag:    ct.Tag,
		ColTag: ct,
		ManifestLists: []ManifestList{
			FT_arm_Manifestlist(),
		},
	}
}

func FT_amd64_Collection() Collection {
	ct := CollectionTag_amd64()
	return Collection{
		Repo:   "qnib/featuretest",
		Tag:    ct.Tag,
		ColTag: ct,
		ManifestLists: []ManifestList{
			FT_amd_Manifestlist(),
		},
	}
}
