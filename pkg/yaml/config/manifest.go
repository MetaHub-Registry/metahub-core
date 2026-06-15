package config

import (
	"github.com/opencontainers/go-digest"
	"github.com/regclient/regclient/types/platform"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

type Manifest struct {
	Image    string            `yaml:"image"`
	Digest   string            `yaml:"digest"`
	Platform platform.Platform `yaml:"platform"`
	// Profile is the optional collection-authored profile label (e.g.
	// "cpu#zen2") this manifest belongs to. When set, materialization
	// binds the manifest to that profile authoritatively (see
	// ProfileInjectDeterministicAnnotations). Empty means fall back to
	// image-annotation / generic matching — the pre-existing behaviour.
	Profile     string            `yaml:"profile,omitempty"`
	Annotations map[string]string `yaml:"annotations"`
}

func (m Manifest) ToUpstreamTriple() (u upstream.UpstreamTriple, err error) {
	reg, repo, tag, err := utils.SplitImageName(m.Image)
	if err != nil {
		return
	}
	u = upstream.UpstreamTriple{
		Reg:    reg,
		Repo:   repo,
		Tag:    tag,
		Digest: digest.Digest(m.Digest),
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

func (m Manifest) ToUpstream() (up types.Upstream, err error) {
	reg, repo, tag, err := utils.SplitImageName(m.Image)
	if err != nil {
		return
	}
	up = types.Upstream{
		Reg:    reg,
		Repo:   repo,
		Tag:    tag,
		Digest: digest.Digest(m.Digest),
		Platform: platform.Platform{
			Architecture: m.Platform.Architecture,
			OS:           m.Platform.OS,
			Variant:      m.Platform.Variant,
			Features:     m.Platform.Features,
		},
	}
	if m.Digest != "" {
		up.Digest = digest.Digest(m.Digest)
	}
	return
}
