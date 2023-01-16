package types

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/docker/distribution"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

type BlobReference struct {
	NeoID     int64  `json:"neo_id"`
	SqlID     int64  `json:"sql_id"`
	Data      []byte `json:"-"`
	Digest    string `json:"digest"`
	Label     string `json:"label"`
	MediaType string `json:"media_type"`
	Size      int64  `json:"size"`
}

func NewBlobReferenceFromBytes(data []byte) (res BlobReference, err error) {
	l := logrus.WithFields(logrus.Fields{
		"packrusage": "types/blob_reference.go",
		"func":       "NewBlobReferenceFromBytes",
	})
	err = json.Unmarshal(data, &res)
	if err != nil {
		l.Error(err)
	}
	res.Data = data
	return
}

func (br BlobReference) ToDescriptor() distribution.Descriptor {
	return distribution.Descriptor{
		Digest:    digest.Digest(br.Digest),
		MediaType: br.MediaType,
		Size:      br.Size,
	}
}

func (br BlobReference) WriteWWW(w http.ResponseWriter) {
	b := br.Data
	size := len(b)
	// TODO: Check if we need to set the Content-Type
	w.Header().Set("Content-Type", br.MediaType)
	w.Header().Set("Content-Length", fmt.Sprint(size))
	w.Header().Set("Docker-Content-Digest", br.Digest)
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	w.Header().Set("Etag", br.Digest)
	w.Write(br.Data)
}
