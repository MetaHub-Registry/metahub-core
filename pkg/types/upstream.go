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

func (u Upstream) ShortString() string {
	if u.Digest != "" {
		return u.Reg + "/" + u.Repo + "@" + utils.ShortenDigest(u.Digest.String())
	}
	return u.Reg + "/" + u.Repo + ":" + u.Tag
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

func NewUpstreamFromRequest(req Request) (up Upstream, err error) {
	up = Upstream{
		Reg:       req.Host,
		Repo:      req.Repo,
		Tag:       req.Tag,
		Profile:   req.Profile,
		MediaType: req.ContentType,
		Size:      int64(req.Length),
	}
	if req.Digest != "" {
		dig, err := digest.Parse(req.Digest)
		if err != nil {
			return up, fmt.Errorf("failed to parse digest '%s': %s", req.Digest, err)
		}
		up.Digest = dig
	}
	return up, nil
}

func (up Upstream) GetBlobUrl(dig ...string) string {
	base := fmt.Sprintf("https://%s/v2/%s/blobs", up.Reg, up.Repo)
	if len(dig) == 0 {
		return base + "/" + up.Digest.String()
	}
	return base + "/" + dig[0]
}

// AllowsUnauthorizedPull returns a static default for this registry.
//
// Deprecated: This method uses a hardcoded registry list. Callers should
// prefer Magazine.AllowsUnauthorizedPull which uses the structured
// RegistryConfig from MagConfig.Registries for config-driven lookups.
func (up Upstream) AllowsUnauthorizedPull() bool {
	switch up.Reg {
	case "registry-1.docker.io", "docker.io":
		return false
	case "quay.io":
		return true
	case "registry:5050":
		return true
	default:
		return false
	}
}
