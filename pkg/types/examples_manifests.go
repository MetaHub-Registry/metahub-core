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

// Zen Featuretest
/* {
"schemaVersion": 2,
"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
"config": {
  "mediaType": "application/vnd.docker.container.image.v1+json",
  "size": 2578,
  "digest": "sha256:c259a14d938a06e8919c53ac6e9c508fb1873f17e579bdcb0810ffafa59fbdf1"
},
"layers": [{
    "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
    "size": 7337984,
    "digest": "sha256:f1417ff83b319fbdae6dd9cd6d8c9c88002dcd75ecf6ec201c8c6894681cf2b5"
  },{
    "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
    "size": 3072,
    "digest": "sha256:4c85de3e6e41d6a5fc11e77e87cd57cddcfa84003e12c93fd62075d924da2636"
*/

func ManifestDataDTFeaturetestZen() DataTuple {
	dt := NewDataTuple("data/manifests/5c99f60c23dfdb5a01cd2268406a8f5616187121655bf66dbad552fbc8530847.json")
	dt.Upstream = Upstream{
		Reg:    "registry.gitlab.com",
		Repo:   "qnib-pub-containers/qnib/plain-featuretest",
		Tag:    "3.17.3-amd64-cpu_zen",
		Digest: dt.Digest,
	}
	return dt
}

var ManifestDataFeaturetestArm64 = newManifestData(
	ManifestDataDTFeaturetestArm64(),
	DataTupeFTConfigArm64(),
	Layer0FTARM64DT,
	Layer1FTARM64DT,
)

func ManifestDataDTFeaturetestArm64() DataTuple {
	dt := NewDataTuple("data/manifests/ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5.json")
	dt.Upstream = Upstream{
		Reg:    "registry.gitlab.com",
		Repo:   "qnib-pub-containers/qnib/plain-featuretest",
		Tag:    "3.17.3-arm64",
		Digest: dt.Digest,
	}
	return dt
}

func ManifestDataAmd64Zen() ManifestData {
	md := newManifestData(
		ManifestDataDTFeaturetestZen(),
		ConfigDTFeaturetestZenc259(),
		LayerDTFeaturetestLf144(),
		LayerDTFeaturetestL4c85(),
	)
	return md
}

// Others
func ManifestDataAmd64Byte() DataTuple {
	return NewDataTuple("data/manifests/5bf8eee20fde14eb6e3e924c18fafa4d816e131f315810875136f0438ff7beed.json")
}

func ManifestDataByteZen3() DataTuple {
	return NewDataTuple("data/manifests/5e0d3e9bb9adff191e78f13dc6e79626bf29000648f0fa94ec5a028462ae5e89.json")
}

func newManifestData(md, cDT DataTuple, lyrs ...DataTuple) ManifestData {
	cfg := newCofigFromDT(cDT)
	ls := []Layer{}
	for _, l := range lyrs {
		ls = append(ls, newLayerFromDT(l))
	}
	mani := ManifestData{
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

func FT_amd_ManifestData() ManifestData {
	dt := ManifestDataAmd64Byte()

	cfg := FT_amd64_Config()
	l0 := FT_amd_Layer0Reference()
	l1 := FT_amd_Layer1Reference()
	up := FT_amd_Upstream()
	mani := ManifestData{
		Digest:    string(digest.FromBytes(dt.Data)),
		Size:      735,
		MediaType: "application/vnd.docker.distribution.manifest.v2+json",
		Data:      dt.Data,
		Config:    cfg,
		Upstream:  up,
		Platform: Platform{
			Arch:    cfg.Cfg.Architecture,
			OS:      cfg.Cfg.OS,
			Variant: cfg.Cfg.Variant,
		},
		Layers: []Layer{
			{Digest: l0.Digest.String(), Size: l0.Size, MediaType: l0.MediaType, Data: FT_amd_Layer0Byte()},
			{Digest: l1.Digest.String(), Size: l1.Size, MediaType: l1.MediaType, Data: FT_amd_Layer1Byte()},
		},
	}
	return mani
}

func FT_zen3_ManifestData() ManifestData {
	dat, err := f.ReadFile("data/manifests/5e0d3e9bb9adff191e78f13dc6e79626bf29000648f0fa94ec5a028462ae5e89.json")
	if err != nil {
		panic(err)
	}
	as := NewAnnotations()
	z3 := NewAnnotation("org.supercontainers.hardware.cpu.target.microarch", "zen3")
	as.AddAnnotation(z3)
	cfg := FT_zen3_Config()
	l0 := FT_zen3_Layer0Reference()
	l1 := FT_zen3_Layer1Reference()
	up := FT_zen3_Upstream()
	mani := ManifestData{
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
			{Digest: l0.Digest.String(), Size: l0.Size, MediaType: l0.MediaType, Data: FT_amd_Layer0Byte()},
			{Digest: l1.Digest.String(), Size: l1.Size, MediaType: l1.MediaType, Data: FT_amd_Layer1Byte()},
		},
	}
	return mani
}

func FT_arm_ManifestData() ManifestData {
	dat, err := f.ReadFile("data/manifests/ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5.json")
	if err != nil {
		panic(err)
	}
	cfg := FT_arm_Config()
	l0 := FT_arm_Layer0Reference()
	l1 := FT_arm_Layer1Reference()
	up := FT_arm_Upstream()
	mani := ManifestData{
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
			{Digest: l0.Digest.String(), Size: l0.Size, MediaType: l0.MediaType, Data: FT_arm_Layer0Byte()},
			{Digest: l1.Digest.String(), Size: l1.Size, MediaType: l1.MediaType, Data: FT_arm_Layer1Byte()},
		},
	}
	return mani
}
