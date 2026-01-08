package upstream

import (
	"encoding/json"
	"fmt"

	digestLib "github.com/opencontainers/go-digest"
	"github.com/regclient/regclient/types/platform"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

type UpstreamTriple struct {
	Reg       string            `json:"reg"`
	Repo      string            `json:"repo"`
	Tag       string            `json:"tag"`
	MediaType string            `json:"mediaType"`
	Size      int64             `json:"size"`
	Profile   string            `json:"profile"`
	Digest    digestLib.Digest  `json:"digest"`
	Platform  platform.Platform `json:"platform"`
}

func NewUpstreamTriple(k map[string]any) (up UpstreamTriple, err error) {
	return NewUpstreamTripleBase(k["reg"].(string), k["repo"].(string), k["tag"].(string), k["mediaType"].(string), k["digest"].(string), k["size"].(int64))
}

func NewUpstreamTripleBase(reg, repo, tag, mediaType, digest string, size int64) (up UpstreamTriple, err error) {
	d, e := digestLib.Parse(digest)
	if e != nil {
		err = e
		return
	}
	up = UpstreamTriple{
		Reg:       reg,
		Repo:      repo,
		Tag:       tag,
		MediaType: mediaType,
		Size:      size,
		Digest:    d,
	}
	return
}

func (upt UpstreamTriple) GetLabel() string {
	return fmt.Sprintf("%s/%s:%s", upt.Reg, upt.Repo, upt.Tag)
}

func (upt UpstreamTriple) GetObjPath() string {
	// Storage path for manifest in Objectstore
	return fmt.Sprintf("%s/%s/%s", upt.Reg, upt.Repo, upt.Tag)
}

func (upt UpstreamTriple) ToBytes() []byte {
	b, _ := json.Marshal(upt)
	return b
}

func UpstreamToUpstreamTriple(up types.Upstream, mediaType string) (upt UpstreamTriple, err error) {
	upt = UpstreamTriple{
		Reg:       up.Reg,
		Repo:      up.Repo,
		Tag:       up.Tag,
		MediaType: mediaType,
		Digest:    up.Digest,
	}
	return
}
