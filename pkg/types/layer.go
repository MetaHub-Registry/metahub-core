package types

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing/fstest"

	"github.com/docker/distribution"
	"github.com/h2non/filetype"
	"github.com/minio/minio-go/v7/pkg/set"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgNameLayer = "types/layer.go"
)

type Layer struct {
	ID        int64  `json:"id"`
	NeoID     int64  `json:"neo_id"`
	SqlID     int64  `json:"sql_id"`
	Data      []byte `json:"-"`
	Digest    string `json:"digest"`
	MediaType string `json:"mediaType"`
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

func LayerFromDesc(ctx context.Context, lyrDesc distribution.Descriptor) (lyr Layer, err error) {
	_, span := tracer.Start(ctx, "LayerFromDesc")
	defer span.End()
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameLayer,
		"func":    "LayerFromDesc",
		"digest":  lyrDesc.Digest.String(),
	})
	// TODO: Add annotations
	_ = l
	lyr.Digest = lyrDesc.Digest.String()
	lyr.MediaType = lyrDesc.MediaType
	lyr.Size = lyrDesc.Size
	return
}

func (lyr Layer) FindFile(ctx context.Context, fname string) (cnt string, found bool, err error) {
	fname = strings.TrimPrefix(fname, "/")
	fs, err := lyr.Uncompress(ctx, fname)
	if err != nil {
		return
	}
	f, ok := fs[fname]
	if !ok {
		return
	}
	found = true
	cnt = string(f.Data)
	return
}

func (lyr Layer) EvalOS(ctx context.Context) (os OS, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameLayer,
		"func":    "types.EvalOS",
	})
	defer span.End()
	fs, found, e := lyr.FindFile(ctx, "etc/os-release")
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to uncompress layer")
	}
	if !found {
		err = utils.NewError(l, span, "failed to find /etc/os-release")
		return
	}
	os, e = ParseOS(fs)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to parse /etc/os-release")
		return
	}
	return

}

func (lyr Layer) Uncompress(ctx context.Context, fnames ...string) (fs fstest.MapFS, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameLayer,
		"func":    "types.Uncompress",
	})
	defer span.End()
	_ = ctx
	fn := set.CreateStringSet(fnames...)
	kind, _ := filetype.Match(lyr.Data)
	if kind == filetype.Unknown {
		err = utils.NewError(l, span, "unknown file type %s", kind)
		return
	}
	var tarRead *tar.Reader
	r := bytes.NewReader(lyr.Data)
	switch kind.MIME.Value {
	case "application/x-tar":
		tarRead = tar.NewReader(r)
	case "application/gzip":
		gzRead, e := gzip.NewReader(r)
		if e != nil {
			return nil, utils.NewError(l, span, "error reading gzip")
		}
		tarRead = tar.NewReader(gzRead)
	default:
		err = utils.NewError(l, span, "unknown file type: %s", kind.MIME.Value)
		return
	}
	fs = make(fstest.MapFS)
	for {
		cur, e := tarRead.Next()
		if e == io.EOF {
			break
		} else if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to read tar")
			return nil, err
		}
		if !fn.IsEmpty() && !fn.Contains(cur.Name) {
			continue
		}
		if cur.Typeflag != tar.TypeReg {
			continue
		}
		data, e := io.ReadAll(tarRead)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to ReadAll")
			return nil, err
		}
		fs[cur.Name] = &fstest.MapFile{Data: data}
	}
	return fs, nil
}
