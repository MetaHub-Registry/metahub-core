package config

import "github.com/metahub-registry/metahub-core/pkg/types"

type ManipuLayer struct {
	Image string `yaml:"image"`
	Rule  string `yaml:"rule"`
}

func (cmpl ManipuLayer) ToTypesManipuLayer() (mpl types.ManipuLayer) {
	up, err := types.NewUpstreamByName(cmpl.Image)
	if err != nil {
		panic(err)
	}
	mpl = types.ManipuLayer{
		Label: cmpl.Image,
		Rule:  cmpl.Rule,
		Image: up,
	}
	return mpl
}
