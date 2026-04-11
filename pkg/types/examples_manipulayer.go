package types

var ManipuLayerGlow150ARM64 = ManipuLayer{
	Label: "layer/glow#150-arm64",
	Image: ManifestGlow150ARM64,
	Rule:  "arch:arm64=true",
}

var ManipuLayerGlow151ARM64 = ManipuLayer{
	Label: "layer/glow#151-arm64",
	Image: ManifestGlow151ARM64,
	Rule:  "arch:arm64=true",
}
