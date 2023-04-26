package types

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/docker/distribution"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

type Layer struct {
	ID        int64  `json:"id"`
	NeoID     int64  `json:"neo_id"`
	SqlID     int64  `json:"sql_id"`
	Data      []byte `json:"-"`
	Digest    string `json:"digest"`
	MediaType string `json:"media_type"`
	Size      int64  `json:"size"`
}

func (l Layer) ToDescriptor() distribution.Descriptor {
	return distribution.Descriptor{
		Digest:    digest.Digest(l.Digest),
		MediaType: l.MediaType,
		Size:      l.Size,
	}
}

func (l Layer) WriteWWWHeader(w http.ResponseWriter) {
	// FIXME: Not sure if that is stable over time.
	w.Header().Set("Docker-Content-Digest", l.Digest)
	if w.Header().Get("Content-Length") == "" {
		w.Header().Set("Content-Length", "0")
	}
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	w.WriteHeader(http.StatusOK)
}

func (l Layer) WriteWWW(w http.ResponseWriter) {
	b := l.Data
	size := len(b)
	w.Header().Set("Docker-Content-Digest", l.Digest)
	w.Header().Set("Content-Length", fmt.Sprint(size))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(l.Data)
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	w.WriteHeader(http.StatusOK)
}

func NewLayerFromBytes(data []byte) (res Layer, err error) {
	l := logrus.WithFields(logrus.Fields{
		"packrusage": "types/config.go",
		"func":       "NewLayerFromBytes",
	})
	err = json.Unmarshal(data, &res)
	if err != nil {
		l.Error(err)
	}
	res.Data = data
	return
}

func Layer_FromDesc(ctx context.Context, lyrDesc distribution.Descriptor) (lyr Layer, err error) {
	_, span := tracer.Start(ctx, "Layer_FromDesc")
	defer span.End()
	l := logrus.WithFields(logrus.Fields{
		"package": "types/layer.go",
		"func":    "Layer_FromDesc",
		"digest":  lyrDesc.Digest.String(),
	})
	// TODO: Add annotations
	_ = l
	lyr.Digest = lyrDesc.Digest.String()
	lyr.MediaType = lyrDesc.MediaType
	lyr.Size = lyrDesc.Size
	return
}
