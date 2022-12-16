package types

import (
	"github.com/opencontainers/go-digest"
)

func FT_amd_Manifestlist() ManifestList {
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
			FT_amd_Manifest(),
		},
	}
}

func FT_generic_Manifestlist() ManifestList {
	dat, err := f.ReadFile("data/manifestlists/451af211ab5d4bb4cf956ec8bf50e9cf57c2083c9dbbcd0c8facf4642cb76932.dat")
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
		Manifests: []Manifest{
			FT_arm_Manifest(),
			FT_amd_Manifest(),
		},
	}
}

func FT_arm_Manifestlist() ManifestList {
	dat, err := f.ReadFile("data/manifestlists/15e01c3caff37bd3e6e4c77d7fa1d1a5afad482d50400b31d20458b792dcb61d.dat")
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
		Manifests: []Manifest{
			FT_arm_Manifest(),
		},
	}
}
