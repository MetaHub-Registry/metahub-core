package config

var ManipuLayerGlow150ARM64 = ManipuLayer{
	Image: "registry:5050/cqnib/metahub-layers/hpc3/glow:1.5.0-arm64",
	Rule:  "arch:arm64=true",
}
var ManipuLayerGlow150AMD64 = ManipuLayer{
	Image: "registry:5050/cqnib/metahub-layers/hpc3/glow:1.5.0-amd64",
	Rule:  "arch:arm64=true",
}
