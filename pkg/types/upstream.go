package types

import (
	"fmt"

	"github.com/opencontainers/go-digest"
	"github.com/regclient/regclient/types/platform"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

type Upstream struct {
	ID        int64             `json:"id"`
	Reg       string            `json:"reg"`
	Repo      string            `json:"repo"`
	Tag       string            `json:"tag"`
	Profile   string            `json:"profile"`
	MediaType string            `json:"mediaType"`
	Size      int64             `json:"size"`
	Digest    digest.Digest     `json:"digest"`
	Platform  platform.Platform `json:"platform"`
}

func (u Upstream) String() string {
	if u.Digest != "" {
		return u.Reg + "/" + u.Repo + "@" + u.Digest.String()
	}
	return u.Reg + "/" + u.Repo + ":" + u.Tag
}

func (upt Upstream) GetLabel() string {
	return fmt.Sprintf("%s/%s:%s", upt.Reg, upt.Repo, upt.Tag)
}

func NewUpstreamByName(n string) (up Upstream, err error) {
	reg, repo, ref, err := utils.SplitImageName(n)
	if err != nil {
		return up, fmt.Errorf("failed to extract upstream from '%s': %s", n, err)
	}
	return Upstream{
		Reg:  reg,
		Repo: repo,
		Tag:  ref,
	}, nil
}
