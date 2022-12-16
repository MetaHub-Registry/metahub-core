package types

import "github.com/opencontainers/go-digest"

type Upstream struct {
	ID     int64         `json:"id"`
	Reg    string        `json:"reg"`
	Repo   string        `json:"repo"`
	Tag    string        `json:"tag"`
	Digest digest.Digest `json:"digest"`
}
