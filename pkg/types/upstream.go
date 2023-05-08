package types

import (
	"fmt"

	"github.com/opencontainers/go-digest"
)

type Upstream struct {
	ID      int64         `json:"id"`
	Reg     string        `json:"reg"`
	Repo    string        `json:"repo"`
	Tag     string        `json:"tag"`
	Profile string        `json:"profile"`
	Digest  digest.Digest `json:"digest"`
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
