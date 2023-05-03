package types

import (
	"embed"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
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
	Example_PlatformAMD64 = []Platform{
		{Arch: "amd64", Variant: "", OS: "linux"},
	}
)

type DataTuple struct {
	Data   []byte
	Digest digest.Digest
}

func NewDataTuple(p string) DataTuple {
	fname := filepath.Base(p)
	ext := filepath.Ext(fname)
	digStr := fmt.Sprintf("sha256:%s", strings.TrimSuffix(fname, ext))
	dat, err := f.ReadFile(p)
	if err != nil {
		panic(err)
	}
	dig := digest.FromBytes(dat)
	if digStr != dig.String() {
		logrus.Fatalf("digest does not match: '%s' != '%s'", digStr, dig.String())
	}
	return DataTuple{Data: dat, Digest: dig}
}

func Example_Platform_amd64() Platform {
	return Platform{
		Arch:    "amd64",
		OS:      "linux",
		Variant: "",
	}
}

func FT_amd_Upstream() Upstream {
	return Upstream{
		Reg:    "registry.gitlab.com",
		Repo:   "qnib/plain-featuretest",
		Tag:    "20220715-amd64-generic",
		Digest: digest.Digest("sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5"),
	}
}

func FT_zen3_Upstream() Upstream {
	return Upstream{
		Reg:    "registry.gitlab.com",
		Repo:   "qnib-pub-containers/qnib/plain-featuretest",
		Tag:    "3-17-3-amd64-cpu_zen3",
		Digest: digest.Digest("sha256:927c3c3f99b1ac9432187ce831541fcf6fca81c051dfdb87fbab303e1a25e780"),
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
