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
			FT_arm_Manifest(),
		},
	}
}

func FT_generic_Manifestlist() ManifestList {
	dat, err := f.ReadFile("data/manifestlists/ABC.dat")
	if err != nil {
		panic(err)
	}
	dig := digest.FromBytes(dat).String()
	if dig != "sha256:ABC" {
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
