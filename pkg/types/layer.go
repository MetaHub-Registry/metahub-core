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
	"path/filepath"
	"strings"
	"testing/fstest"

	"github.com/docker/distribution"
	"github.com/h2non/filetype"
	"github.com/minio/minio-go/v7/pkg/set"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/trace"
)

const (
	pkgNameLayer = "types/layer.go"
)

type Layer struct {
	ID          int64             `json:"id"`
	NeoID       int64             `json:"neo_id"`
	SqlID       int64             `json:"sql_id"`
	Data        []byte            `json:"-"`
	Digest      string            `json:"digest"`
	MediaType   string            `json:"mediaType"`
	Size        int64             `json:"size"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Platform    *Platform         `json:"platform,omitempty"`
}

func (l Layer) ToDescriptor() distribution.Descriptor {
	return distribution.Descriptor{
		Digest:      digest.Digest(l.Digest),
		MediaType:   l.MediaType,
		Size:        l.Size,
		Annotations: l.Annotations,
	}
}

func (l Layer) WriteWWWHeader(w http.ResponseWriter) {
	// FIXME: Not sure if that is stable over time.
	w.Header().Set(DockerContentDigest, l.Digest)
	if w.Header().Get(ContentLength) == "" {
		w.Header().Set(ContentLength, "0")
	}
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	w.WriteHeader(http.StatusOK)
}

func (l Layer) WriteWWW(w http.ResponseWriter) {
	b := l.Data
	size := len(b)
	w.Header().Set(DockerContentDigest, l.Digest)
	w.Header().Set(ContentLength, fmt.Sprint(size))
	w.Header().Set(ContentType, "application/octet-stream")
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
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameLayer,
		"func":    "LayerFromDesc",
		"digest":  lyrDesc.Digest.String(),
	})
	defer span.End()
	lyr.Digest = lyrDesc.Digest.String()
	lyr.MediaType = lyrDesc.MediaType
	lyr.Size = lyrDesc.Size
	lyr.Annotations = lyrDesc.Annotations
	utils.Debug(l, span, "Layer has %d annotations", len(lyr.Annotations))
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
		return
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

// maxLinkDepth bounds symlink-chain resolution when materializing link names.
const maxLinkDepth = 8

// newTarReader returns a fresh tar.Reader over the layer's (possibly gzipped)
// data. It is called once per scan pass because a tar.Reader is a stream.
func (lyr Layer) newTarReader(l *logrus.Entry, span trace.Span) (*tar.Reader, error) {
	kind, _ := filetype.Match(lyr.Data)
	if kind == filetype.Unknown {
		return nil, utils.NewError(l, span, "unknown file type %s", kind)
	}
	r := bytes.NewReader(lyr.Data)
	switch kind.MIME.Value {
	case "application/x-tar":
		return tar.NewReader(r), nil
	case "application/gzip":
		gzRead, e := gzip.NewReader(r)
		if e != nil {
			return nil, utils.NewError(l, span, "error reading gzip")
		}
		return tar.NewReader(gzRead), nil
	default:
		return nil, utils.NewError(l, span, "unknown file type: %s", kind.MIME.Value)
	}
}

func (lyr Layer) Uncompress(ctx context.Context, fnames ...string) (fs fstest.MapFS, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameLayer,
		"func":    "types.Uncompress",
	})
	defer span.End()
	_ = ctx
	fn := set.CreateStringSet(fnames...)

	// Pass 1 (filtered scans only): resolve requested names that ship as
	// symlink/hardlink entries — e.g. alpine >=3.21 ships /etc/os-release as
	// a symlink to /usr/lib/os-release — to their tar-internal target path.
	links := map[string]string{}
	if !fn.IsEmpty() {
		tr, e := lyr.newTarReader(l, span)
		if e != nil {
			return nil, e
		}
		links, e = scanLinkTargets(tr, fn, l, span)
		if e != nil {
			return nil, e
		}
	}
	// Pass 2: read the requested regular files plus the link targets.
	targets := make(map[string]bool, len(links))
	for _, t := range links {
		targets[t] = true
	}
	tr, e := lyr.newTarReader(l, span)
	if e != nil {
		return nil, e
	}
	fs, e = readRegularFiles(tr, fn, targets, l, span)
	if e != nil {
		return nil, e
	}
	materializeLinks(fs, links)
	return fs, nil
}

// scanLinkTargets walks tar headers and records requested symlink/hardlink
// names, each resolved to its tar-internal target path.
func scanLinkTargets(tr *tar.Reader, fn set.StringSet, l *logrus.Entry, span trace.Span) (map[string]string, error) {
	links := make(map[string]string)
	for {
		cur, e := tr.Next()
		if e == io.EOF {
			return links, nil
		}
		if e != nil {
			return nil, utils.ErrorHandler(l, span, e, "failed to read tar")
		}
		if !isLinkEntry(cur) || !fn.Contains(cur.Name) {
			continue
		}
		links[cur.Name] = resolveLinkTarget(cur)
	}
}

// isLinkEntry reports whether the tar header is a symlink or hardlink.
func isLinkEntry(h *tar.Header) bool {
	return h.Typeflag == tar.TypeSymlink || h.Typeflag == tar.TypeLink
}

// resolveLinkTarget normalizes a link's target to a tar-internal path:
// relative symlink targets are resolved against the entry's directory,
// absolute paths lose their leading slash.
func resolveLinkTarget(h *tar.Header) string {
	target := h.Linkname
	if h.Typeflag == tar.TypeSymlink && !strings.HasPrefix(target, "/") {
		target = filepath.Join(filepath.Dir(h.Name), target)
	}
	return strings.TrimPrefix(target, "/")
}

// readRegularFiles copies the content of regular-file entries whose name is
// requested (or is a link target) into a MapFS. An empty request set reads
// every regular file, preserving the unfiltered-scan behavior.
func readRegularFiles(tr *tar.Reader, fn set.StringSet, targets map[string]bool, l *logrus.Entry, span trace.Span) (fstest.MapFS, error) {
	fs := make(fstest.MapFS)
	for {
		cur, e := tr.Next()
		if e == io.EOF {
			return fs, nil
		}
		if e != nil {
			return nil, utils.ErrorHandler(l, span, e, "failed to read tar")
		}
		if cur.Typeflag != tar.TypeReg {
			continue
		}
		if !fn.IsEmpty() && !fn.Contains(cur.Name) && !targets[cur.Name] {
			continue
		}
		data, e := io.ReadAll(tr)
		if e != nil {
			return nil, utils.ErrorHandler(l, span, e, "failed to ReadAll")
		}
		fs[cur.Name] = &fstest.MapFile{Data: data}
	}
}

// materializeLinks fills requested names that exist only as symlink/hardlink
// entries with the content of the regular file they point at, following link
// chains up to maxLinkDepth hops (loop-guarded).
func materializeLinks(fs fstest.MapFS, links map[string]string) {
	for name, target := range links {
		if _, ok := fs[name]; ok {
			continue
		}
		cur := target
		for i := 0; i < maxLinkDepth; i++ {
			if data, ok := fs[cur]; ok {
				fs[name] = data
				break
			}
			next, ok := links[cur]
			if !ok {
				break
			}
			cur = next
		}
	}
}
