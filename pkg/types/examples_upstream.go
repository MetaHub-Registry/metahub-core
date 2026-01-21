package types

import "github.com/opencontainers/go-digest"

const (
	reg5050 = "registry:5050"
)

var UpstreamGlow150ARM64 = Upstream{
	Reg:    reg5050,
	Repo:   "cqnib/metahub-layers/hpc3/glow",
	Tag:    "1.5.0-arm64",
	Digest: "sha256:3aabfa0b0457345ef833ca640823da969b0c43e542caf10c497e37a85213df60",
}

var UpstreamGlow151ARM64 = Upstream{
	Reg:    reg5050,
	Repo:   "cqnib/metahub-layers/hpc3/glow",
	Tag:    "1.5.1-arm64",
	Digest: "sha256:7b8f26f6ed712db19b9fd2f8fffff972c22defdf0e5176a09e2842f9a9b01fef",
}

var UpstreamGlow150AMD64 = Upstream{
	Reg:    reg5050,
	Repo:   "cqnib/metahub-layers/hpc3/glow",
	Tag:    "1.5.0-amd64",
	Digest: "sha256:e390c871ff920a63eab82556c14f250f64667d186af3fc8ac976e02da61f069d",
}

func FT_zen3_Upstream() Upstream {
	return Upstream{
		Reg:    reg5050,
		Repo:   "qnib-pub-containers/qnib/plain-featuretest",
		Tag:    "3-17-3-amd64-cpu_zen3",
		Digest: digest.Digest("sha256:927c3c3f99b1ac9432187ce831541fcf6fca81c051dfdb87fbab303e1a25e780"),
	}
}

func FT_amd64_Upstream() Upstream {
	return Upstream{
		Reg:    reg5050,
		Repo:   "qnib-pub-containers/qnib/plain-featuretest",
		Tag:    "20220715-amd64-generic",
		Digest: digest.Digest("sha256:5bf8eee20fde14eb6e3e924c18fafa4d816e131f315810875136f0438ff7beed"),
	}
}

func FT_arm64_Upstream() Upstream {
	dt := NewDataTuple("data/manifests/ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5.json")
	return Upstream{
		Reg:    reg5050,
		Repo:   "qnib-pub-containers/qnib/plain-featuretest",
		Tag:    "3.17.3-arm64",
		Digest: dt.Digest,
	}
}
