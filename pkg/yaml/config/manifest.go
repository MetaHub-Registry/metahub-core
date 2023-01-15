package config

import (
	"github.com/opencontainers/go-digest"
	"github.com/regclient/regclient/types/platform"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

type Manifest struct {
	Image    string            `yaml:"image"`
	Digest   string            `yaml:"digest"`
	Platform platform.Platform `yaml:"platform"`
	//Annotations map[string]string `yaml:"annotations"`
}

func (m Manifest) ToUpstreamTriple() (u upstream.UpstreamTriple, err error) {
	reg, repo, tag, err := utils.SplitImageName(m.Image)
	if err != nil {
		return
	}
	u = upstream.UpstreamTriple{
		Reg:  reg,
		Repo: repo,
		Tag:  tag,
		Platform: platform.Platform{
			Architecture: m.Platform.Architecture,
			OS:           m.Platform.OS,
			Variant:      m.Platform.Variant,
			Features:     m.Platform.Features,
		},
	}
	if m.Digest != "" {
		u.Digest = digest.Digest(m.Digest)
	}
	return
}
