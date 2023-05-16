package types

var ManipuLayerGlow150ARM64 = ManipuLayer{
	Label:  "layer/glow#150-arm64",
	Image:  UpstreamGlow150ARM64,
	Rule:   "arch:arm64=true",
	Layers: []Layer{},
}
