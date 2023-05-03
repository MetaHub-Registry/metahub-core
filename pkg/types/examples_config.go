package types

import (
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func DataTupeFTConfigArm64() DataTuple {
	return NewDataTuple("data/cfg/cf5c26263929e84d7373d23b2692222cceceb3175b3325b183ead2ab6b5a5ce3.json")
}

func ConfigByteFtArm64() []byte {
	// read file and return bytes
	dt := DataTupeFTConfigArm64()
	return dt.Data
}

func FT_amd64_ConfigByte() []byte {
	// read file and return bytes
	dat, err := f.ReadFile("data/cfg/fba3e3993bfe856a766b683c73e1ddeb03f2d359d7bd7700095682d94c3fcebf.json")
	if err != nil {
		panic(err)
	}
	return dat
}

func FT_amd64_b109_ConfigByte() []byte {
	// read file and return bytes
	dat, err := f.ReadFile("data/cfg/b109e1dac65af00234a8a7f817dcb1e18bc18ca97a5b805283995ebf1137e471.json")
	if err != nil {
		panic(err)
	}
	return dat
}

func ConfigByteFtArm64x83f() DataTuple {
	return NewDataTuple("data/cfg/83fdbde95f56beb69853240df571a7f085c561b4f8d914c5cc5e0f171dcb86df.json")
}

func FT_zen3_ConfigByte() []byte {
	dat, err := f.ReadFile("data/cfg/d33f8ced1dadd99fcf8f210cc846322b69e9065ddf10ceb09ff244b624dcbcc2.json")
	if err != nil {
		panic(err)
	}
	return dat
}

func FT_zen3_Config() Config {
	cfg, _ := NewConfigFromBytes(FT_zen3_ConfigByte())
	cfg.MediaType = "application/vnd.docker.container.image.v1+json"
	return cfg
}

func ExmplConfigAmd64x83f() Config {
	dt := ConfigByteFtArm64x83f()
	cfg, _ := NewConfigFromBytes(dt.Data)
	cfg.Data = dt.Data
	cfg.MediaType = "application/vnd.docker.container.image.v1+json"
	return cfg
}
func FT_amd64_Config() Config {
	cfg, _ := NewConfigFromBytes(FT_amd64_ConfigByte())
	cfg.MediaType = "application/vnd.docker.container.image.v1+json"
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
	cfg, _ := NewConfigFromBytes(ConfigByteFtArm64())
	cfg.MediaType = "application/vnd.docker.container.image.v1+json"
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
