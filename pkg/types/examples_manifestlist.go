package types

import (
	"github.com/opencontainers/go-digest"
)

func FT_amd_Manifestlist() ManifestList {
	dat, err := f.ReadFile("data/manifestlists/d46f2503b600eac6066ba46c034425096087875de9480334b3471347d14bf227.json")
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
		Manifests: []ManifestData{
			FT_amd_ManifestData(),
		},
	}
}

func FTOCIManifestlist() ManifestList {
	dat, err := f.ReadFile("data/manifestlists/oci/508f21b8552552157ea35e86da67e4de414a5f36b9873f077e05678a125908d8.json")
	if err != nil {
		panic(err)
	}
	dig := digest.FromBytes(dat).String()
	if dig != "sha256:508f21b8552552157ea35e86da67e4de414a5f36b9873f077e05678a125908d8" {
		panic("digest mismatch")
	}
	return ManifestList{
		SchemaVersion: 2,
		Digest:        dig,
		MediaType:     "application/vnd.oci.image.index.v1+json",
		Manifests:     []ManifestData{},
	}
}

func FT_generic_Manifestlist() ManifestList {
	dat, err := f.ReadFile("data/manifestlists/451af211ab5d4bb4cf956ec8bf50e9cf57c2083c9dbbcd0c8facf4642cb76932.json")
	if err != nil {
		panic(err)
	}
	dig := digest.FromBytes(dat).String()
	if dig != "sha256:451af211ab5d4bb4cf956ec8bf50e9cf57c2083c9dbbcd0c8facf4642cb76932" {
		panic("digest mismatch")
	}
	return ManifestList{
		SchemaVersion: 2,
		Digest:        dig,
		MediaType:     "application/vnd.docker.distribution.manifest.list.v2+json",
		Manifests: []ManifestData{
			FT_arm_ManifestData(),
			FT_amd_ManifestData(),
		},
	}
}

func FT_arm_Manifestlist() ManifestList {
	dat, err := f.ReadFile("data/manifestlists/15e01c3caff37bd3e6e4c77d7fa1d1a5afad482d50400b31d20458b792dcb61d.json")
	if err != nil {
		panic(err)
	}

	dig := digest.FromBytes(dat).String()
	if dig != "sha256:15e01c3caff37bd3e6e4c77d7fa1d1a5afad482d50400b31d20458b792dcb61d" {
		panic("digest mismatch")
	}
	return ManifestList{
		SchemaVersion: 2,
		Digest:        dig,
		Size:          len(dat),
		MediaType:     "application/vnd.docker.distribution.manifest.list.v2+json",
		Manifests: []ManifestData{
			FT_arm_ManifestData(),
		},
	}
}
