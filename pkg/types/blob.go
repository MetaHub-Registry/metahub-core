package types

import (
	"fmt"
	"net/http"

	"github.com/docker/distribution"
	"github.com/opencontainers/go-digest"
)

const (
	pkgNameBlob = "types/blob.go"
)

type Blob struct {
	ID        int64  `json:"id"`
	NeoID     int64  `json:"neo_id"`
	SqlID     int64  `json:"sql_id"`
	Data      []byte `json:"-"`
	Digest    string `json:"digest"`
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
}

func (b Blob) ToDescriptor() distribution.Descriptor {
	return distribution.Descriptor{
		Digest:    digest.Digest(b.Digest),
		MediaType: b.MediaType,
		Size:      b.Size,
	}
}

func (b Blob) WriteWWWHeader(w http.ResponseWriter) {
	// FIXME: Not sure if that is stable over time.
	w.Header().Set("Docker-Content-Digest", b.Digest)
	if w.Header().Get("Content-Length") == "" {
		w.Header().Set("Content-Length", "0")
	}
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	w.WriteHeader(http.StatusOK)
}

func (b Blob) WriteWWW(w http.ResponseWriter) {
	data := b.Data
	size := len(data)
	w.Header().Set("Docker-Content-Digest", b.Digest)
	w.Header().Set("Content-Length", fmt.Sprint(size))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(b.Data)
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	w.WriteHeader(http.StatusOK)
}
