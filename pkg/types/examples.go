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
	*/
	Example_PlatformARM64 = []Platform{
		{Arch: "arm64", Variant: "", OS: "linux"},
	}

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

const (
	MediaTypeManifest     = "application/vnd.docker.distribution.manifest.v2+json"
	MediaTypeImageConfig  = "application/vnd.docker.container.image.v1+json"
	MediaTypePluginConfig = "application/vnd.docker.plugin.v1+json"
	MediaTypeLayer        = "application/vnd.docker.image.rootfs.diff.tar.gzip"
	MediaTypeManifestList = "application/vnd.docker.distribution.manifest.list.v2+json"
)

type DataTuple struct {
	Upstream Upstream
	Data     []byte
	Digest   digest.Digest
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
	return DataTuple{
		Upstream: Upstream{Repo: ""},
		Data:     dat,
		Digest:   dig,
	}
}

func ExamplePlatformamd64() Platform {
	return Platform{
		Arch:    "amd64",
		OS:      "linux",
		Variant: "",
	}
}

func CollectionTagamd64() CollectionTag {
	return CollectionTag{Tag: "amd64"}
}

func CollectionTagarm() CollectionTag {
	return CollectionTag{Tag: "arm64"}
}

func CollectionTagGeneric() CollectionTag {
	return CollectionTag{Tag: "generic"}
}

func FTGenericCollection() Repo {
	ct := CollectionTagGeneric()
	return Repo{
		Repo:   "qnib/featuretest",
		Tag:    ct.Tag,
		ColTag: ct,
		ManifestLists: []Index{
			FTarmManifestlist(),
			FTamdManifestlist(),
		},
	}
}

func FTarmCollection() Repo {
	ct := CollectionTagarm()
	return Repo{
		Repo:   "qnib/featuretest",
		Tag:    ct.Tag,
		ColTag: ct,
		ManifestLists: []Index{
			FTarmManifestlist(),
		},
	}
}

func FTamd64Collection() Repo {
	ct := CollectionTagamd64()
	return Repo{
		Repo:   "qnib/featuretest",
		Tag:    ct.Tag,
		ColTag: ct,
		ManifestLists: []Index{
			FTamdManifestlist(),
		},
	}
}
