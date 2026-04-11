package types

import "github.com/opencontainers/go-digest"

var (
	ManifestAmd64Zen = newManifest(
		ManifestDTFeaturetestZen(),
		ConfigDTFeaturetestZenc259(),
		LayerDTFeaturetestLf144(),
		LayerDTFeaturetestL4c85(),
	)
	ManifestFeaturetestArm64 = newManifest(
		ManifestDTFeaturetestArm64(),
		DataTupeFTConfigArm64(),
		Layer0FTARM64DT,
		Layer1FTARM64DT,
	)
	ManifestGlow150ARM64 = newManifest(
		ManifestDTGlow150Arm64(),
		ConfigDTGlow150ARM64,
		Layer0Glow150ARM64DT,
		Layer1Glow150ARM64DT,
		Layer2Glow150ARM64DT,
	)
	ManifestGlow151ARM64 = newManifest(
		ManifestDTGlow151Arm64(),
		ConfigDTGlow151ARM64,
		Layer0Glow151ARM64DT,
		Layer1Glow151ARM64DT,
		Layer2Glow151ARM64DT,
	)
	ManifestOCI = newManifest(
		ManifestDTOCI(),
		ConfigDTOCI,
	)
)

func ManifestDTGlow150Arm64() DataTuple {
	dt := NewDataTuple("data/manifests/3aabfa0b0457345ef833ca640823da969b0c43e542caf10c497e37a85213df60.json")
	dt.Upstream = Upstream{
		Reg:    reg5050,
		Repo:   "cqnib/metahub-layers/hpc3/glow",
		Tag:    "1.5.0-arm64",
		Digest: dt.Digest,
	}
	return dt
}
func ManifestDTGlow151Arm64() DataTuple {
	dt := NewDataTuple("data/manifests/7b8f26f6ed712db19b9fd2f8fffff972c22defdf0e5176a09e2842f9a9b01fef.json")
	dt.Upstream = Upstream{
		Reg:    reg5050,
		Repo:   "cqnib/metahub-layers/hpc3/glow",
		Tag:    "1.5.1-arm64",
		Digest: dt.Digest,
	}
	return dt
}

func newManifest(md, cDT DataTuple, lyrs ...DataTuple) Manifest {
	cfg := newCofigFromDT(cDT)
	ls := []Layer{}
	for _, l := range lyrs {
		ls = append(ls, newLayerFromDT(l))
	}
	mani := Manifest{
		Digest:    string(digest.FromBytes(md.Data)),
		Size:      int64(len(md.Data)),
		MediaType: MediaTypeManifest,
		Data:      md.Data,
		Config:    cfg,
		Upstream:  md.Upstream,
		Platform: Platform{
			Arch:    cfg.Cfg.Architecture,
			OS:      cfg.Cfg.OS,
			Variant: cfg.Cfg.Variant,
		},
		Layers: ls,
	}
	return mani
}

// OLD
func ManifestDTFeaturetestZen() DataTuple {
	dt := NewDataTuple("data/manifests/de7c0dc6914e6554f14857f7b0229d25f4ce12495eec7ecb9ece7a9f4e11b8e8.json")
	dt.Upstream = Upstream{
		Reg:    "registry.gitlab.com",
		Repo:   "qnib-pub-containers/qnib/plain-featuretest",
		Tag:    "3.17.3-amd64-cpu_zen",
		Digest: dt.Digest,
	}
	return dt
}

func ManifestDTFeaturetestArm64() DataTuple {
	dt := NewDataTuple("data/manifests/ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5.json")
	dt.Upstream = Upstream{
		Reg:    "registry:5050",
		Repo:   "qnib-pub-containers/qnib/plain-featuretest",
		Tag:    "3.17.3-arm64",
		Digest: dt.Digest,
	}
	return dt
}

// Others
func ManifestAmd64Byte() DataTuple {
	return NewDataTuple("data/manifests/5bf8eee20fde14eb6e3e924c18fafa4d816e131f315810875136f0438ff7beed.json")
}

func ManifestByteZen3() DataTuple {
	return NewDataTuple("data/manifests/5e0d3e9bb9adff191e78f13dc6e79626bf29000648f0fa94ec5a028462ae5e89.json")
}

func FTamdManifest() Manifest {
	dt := ManifestAmd64Byte()
	cfg := FTamd64Config()
	l0 := FTamdLayer0Reference()
	l1 := FTamdLayer1Reference()
	up := FTamd64Upstream()
	mani := Manifest{
		Digest:    string(digest.FromBytes(dt.Data)),
		Size:      735,
		MediaType: "application/vnd.docker.distribution.manifest.v2+json",
		Data:      dt.Data,
		Config:    cfg,
		Type:      "ORIGINAL",
		Upstream:  up,
		Platform: Platform{
			Arch:    cfg.Cfg.Architecture,
			OS:      cfg.Cfg.OS,
			Variant: cfg.Cfg.Variant,
		},
		Layers: []Layer{
			{Digest: l0.Digest.String(), Size: l0.Size, MediaType: l0.MediaType, Data: FTamdLayer0Byte()},
			{Digest: l1.Digest.String(), Size: l1.Size, MediaType: l1.MediaType, Data: FTamdLayer1Byte()},
		},
	}
	return mani
}

// func FTzen3ManifestWithAnno() Manifest {
// 	dat, err := f.ReadFile("data/manifests/2c3ac7b0420735df507e072b0b1e93640a8ae7396218637e846074e6409da579.json")
// 	if err != nil {
// 		panic(err)
// 	}
// }

func FTzen3Manifest() Manifest {
	dat, err := f.ReadFile("data/manifests/de7c0dc6914e6554f14857f7b0229d25f4ce12495eec7ecb9ece7a9f4e11b8e8.json")
	if err != nil {
		panic(err)
	}
	as := NewAnnotations()
	z3 := NewAnnotation("org.supercontainers.hardware.cpu.target.microarch", "zen3")
	as.AddAnnotation(z3)
	cfg := FTzen3Config()
	l0 := FTzen3Layer0Reference()
	l1 := FTzen3Layer1Reference()
	up := FTzen3Upstream()
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
		Annotations: as,
		Layers: []Layer{
			{Digest: l0.Digest.String(), Size: l0.Size, MediaType: l0.MediaType, Data: FTamdLayer0Byte()},
			{Digest: l1.Digest.String(), Size: l1.Size, MediaType: l1.MediaType, Data: FTamdLayer1Byte()},
		},
	}
	return mani
}

func FTarmManifest() Manifest {
	dat, err := f.ReadFile("data/manifests/ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5.json")
	if err != nil {
		panic(err)
	}
	cfg := FTarmConfig()
	l0 := FTarmLayer0Reference()
	l1 := FTarmLayer1Reference()
	up := FTarm64Upstream()
	mani := Manifest{
		Digest:    string(digest.FromBytes(dat)),
		Size:      735,
		MediaType: "application/vnd.docker.distribution.manifest.v2+json",
		Data:      dat,
		Config:    cfg,
		Type:      "PARSED",
		Upstream:  up,
		Platform: Platform{
			Arch:    cfg.Cfg.Architecture,
			OS:      cfg.Cfg.OS,
			Variant: cfg.Cfg.Variant,
		},

		Layers: []Layer{
			{Digest: l0.Digest.String(), Size: l0.Size, MediaType: l0.MediaType, Data: FTarmLayer0Byte()},
			{Digest: l1.Digest.String(), Size: l1.Size, MediaType: l1.MediaType, Data: FTarmLayer1Byte()},
		},
	}
	return mani
}

/**** OCI ****/
func ManifestDTOCI() DataTuple {
	dt := NewDataTuple("data/manifests/oci/d701f04d63badebf6532354e76d8d36fdba8fed2c5f1ab538cb1847baab3de22.json")
	dt.Upstream = Upstream{
		Reg:    "registry:5050",
		Repo:   "qnib/test",
		Tag:    "1.0.0",
		Digest: dt.Digest,
	}
	return dt
}
