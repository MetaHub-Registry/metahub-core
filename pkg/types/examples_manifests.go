package types

import (
	"github.com/opencontainers/go-digest"
)

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
func FT_amd_Manifest() Manifest {
	dat, err := f.ReadFile("data/manifests/ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5.dat")
	if err != nil {
		panic(err)
	}
	cfg := FT_amd_Config()
	l0 := FT_amd_Layer0Reference()
	l1 := FT_amd_Layer1Reference()
	up := FT_amd_Upstream()
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
