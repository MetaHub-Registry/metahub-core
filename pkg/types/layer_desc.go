package types

import (
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type LayerDesc struct {
	LayerID    int64
	Descriptor v1.Descriptor
}

func NewLayerDesc_FromLayer(l Layer) (ld LayerDesc) {
	ld.LayerID = l.ID
	ld.Descriptor = v1.Descriptor{
		MediaType: l.MediaType,
		Digest:    digest.Digest(l.Digest),
		Size:      l.Size,
	}
	return ld
}
