package magazine

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgNameLayer = "magazine/layer.go"
)

func (mag Magazine) PullLayerFromAllRegs(ctx context.Context, up upstream.UpstreamTriple) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameLayer,
		"func":    "mag.PullLayerFromAllRegs",
		"repo":    up.Repo,
		"tag":     up.Tag,
		"digest":  utils.ShortenDigest(up.Digest.String()),
	})
	defer span.End()
	regs, ok := mag.GetConfig("registries")
	if !ok {
		err = utils.NewError(l, span, "Fail to get registries from config")
		return
	}
	for _, reg := range strings.Split(regs, ",") {
		up.Reg = reg
		utils.Debug(l, span, "Using registry: %s", reg)
		e := mag.PullLayer(ctx, up)
		if e != nil {
			utils.Warn(l, span, "Error pulling layer: %v", e.Error())
		} else {
			utils.Debug(l, span, "PullLayer successful - break")
			break
		}
	}
	return

}

func (mag Magazine) PullLayer(ctx context.Context, up upstream.UpstreamTriple) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameLayer,
		"func":    "mag.PullLayer",
		"reg":     up.Reg,
		"repo":    up.Repo,
		"tag":     up.Tag,
		"digest":  utils.ShortenDigest(up.Digest.String()),
	})
	defer span.End()
	rc, blobs, err := mag.GetBlobReaderByUpstreamTripleByDigest(ctx, up, up.Digest.String())
	if err != nil {
		err = errors.Wrapf(err, "failed to get blob reader for %s", up.GetLabel())
		l.Error(err)
		return err
	}
	defer rc.Close()
	_ = blobs
	// Stream upstream -> MinIO at constant memory (D6); no whole-layer buffer.
	err = mag.Minio.PutBlobReader(ctx, rc, up.Digest.String())
	if err != nil {
		logrus.Error(err)
		return err
	}
	return
}

func (mag Magazine) parseCacheLayers() (cacheLayers int, err error) {
	cacheLayers = -1
	if cLyr, ok := mag.GetConfig("cache-layers"); ok {
		switch cLyr {
		case "all":
			cacheLayers = -1
		case "none":
			cacheLayers = 0
		default:
			if cacheLayers, err = strconv.Atoi(cLyr); err != nil {
				return cacheLayers, fmt.Errorf("failed to parse cache-layers: %s", cLyr)
			}
		}
	}
	return
}

func (mag Magazine) CacheLayers(ctx context.Context, mani types.Manifest) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameLayer,
		"func":    "mag.CacheLayers",
	})
	defer span.End()
	up := upstream.UpstreamTriple{
		Reg:  mani.Upstream.Reg,
		Repo: mani.Upstream.Repo,
		Tag:  mani.Upstream.Tag,
	}
	cacheLayer, err := mag.parseCacheLayers()
	if err != nil {
		return utils.ErrorHandler(l, span, err, "failed to parse cache-layers")
	}
	utils.Warn(l, span, "Cache layers: %d", cacheLayer)
	for idx, layer := range mani.Layers {
		if cacheLayer == 0 || (cacheLayer > 0 && idx >= cacheLayer) {
			utils.Info(l, span, " >> Skip caching layer[%-2d] %s for %-90s", idx, utils.ShortenDigest(layer.Digest), up.GetLabel())
			continue
		}
		up.Digest = digest.Digest(layer.Digest)
		isCached, e := mag.Minio.GetBlob(ctx, up)
		if e != nil {
			err = e
			logrus.Error(err)
			return
		}
		utils.Info(l, span, "Caching layer[%-2d][%-20s] %s for %-90s -> %t", idx, utils.ShortenMediaType(layer.MediaType), utils.ShortenDigest(layer.Digest), up.GetLabel(), isCached)
		if !isCached {
			err = mag.PullLayer(ctx, up)
			if err != nil {
				logrus.Error(err)
				return err
			}
		}
	}
	return
}

func (mag Magazine) LayerPlainInsert(ctx context.Context, layer types.Layer) (layerID int64, err error) {
	return mag.Database.LayerPlainInsert(ctx, layer)
}

func (mag Magazine) NeoInsertLayer(ctx context.Context, layer types.Layer) (layerID int64, err error) {
	return mag.LayerPlainInsert(ctx, layer)
}

func (mag Magazine) StoreLayerUpload(ctx context.Context, bu *types.OCIBlobUpload, buf []byte) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":  pkgNameLayer,
		"func":     "mag.BaseLayerEval",
		"blobType": bu.BlobType,
		"digest":   bu.Digest,
	})
	defer span.End()
	err = mag.Minio.PutBlob(ctx, buf, bu.Digest)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "failed to put layer into minio")
	}
	// Insert Layer into SQLite
	lyr := types.Layer{
		Digest:    bu.Digest,
		MediaType: "unknown",
		Size:      int64(len(buf)),
	}
	//err = mag.SQlite.Layer.Upsert(&l)
	_, e := mag.Database.LayerPlainInsert(ctx, lyr)
	if e != nil {
		return utils.ErrorHandler(l, span, e, "failed to insert layer into graph")
	}
	// Link Layer to BlobUpload
	if e := mag.Database.BlobUploadUpdateByUUID(ctx, bu); e != nil {
		return utils.ErrorHandler(l, span, e, "failed to link layer to blob upload")
	}
	return nil
}

// LayerGetPresignedUrlByHTTPRequest mints a presigned GET URL for the blob named
// in the request. ctx is threaded from the caller so the presign carries trace
// context and honours request cancellation (previously a context.Background()
// that dropped both on the pull-redirect hot path).
func (mag Magazine) LayerGetPresignedUrlByHTTPRequest(ctx context.Context, r *http.Request) (psUrl string, err error) {
	req := types.NewRequest(ctx, r)
	return mag.Minio.BlobGetPreSignedUrl(ctx, req.Digest)
}

func (mag Magazine) LayerGetByHTTPRequest(r *http.Request) (layer types.Layer, found bool, err error) {
	ctx := context.Background()
	req := types.NewRequest(ctx, r)
	//logrus.Debugf("LayerGetByHTTPRequest: %s", req.String())
	// Fetch Layer from SQLite
	layer, found, err = mag.Database.LayerGetByDigest(ctx, req.Digest)
	if err != nil {
		logrus.Errorf("failed to fetch layer from SQLite: %v", err)
		return
	}
	if !found {
		logrus.Errorf("did not find layer with dig: %s", req.Digest)
		return
	}
	// Fetch Blob
	b, err := mag.Minio.BlobGetBytes(ctx, req.Digest)
	switch {
	case err != nil && err.Error() == "The specified key does not exist.":
		// found is false
		return
	case err != nil:
		logrus.Error(err)
		return
	}
	layer.Data = b
	// No error - off we go
	found = true
	return
}

func (mag Magazine) GetLayerByOCIManifest(oMani types.OCIManifest) (layers []types.Layer, err error) {
	ctx := context.Background()
	for _, l := range oMani.Layers {
		layer, found, err := mag.Database.LayerGetByDigest(ctx, l.Digest.String())
		if err != nil {
			logrus.Errorf("failed to fetch layer from SQLite: %v", err)
			return layers, err
		}
		if !found {
			err = fmt.Errorf("did not find layer with dig: %s", l.Digest)
			return layers, err
		}
		// Fetch Blob
		b, err := mag.Minio.BlobGetBytes(ctx, l.Digest.String())
		switch {
		case err != nil && err.Error() == "The specified key does not exist.":
			// found is false
			return layers, err
		case err != nil:
			logrus.Error(err)
			return layers, err
		}
		layer.Data = b
		layers = append(layers, layer)
	}
	return layers, nil
}

func (mag Magazine) LayersCreateOCIv1LayerDescriptors(ctx context.Context, layerIn []types.Layer) (res []types.LayerDesc, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameLayer,
		"func":    "LayersCreateOCIv1LayerDescriptors",
	})
	defer span.End()
	res = []types.LayerDesc{}
	var mt string
	for _, ly := range layerIn {
		utils.Debug(l, span, "Layer (%s) with mediatype: %s", utils.ShortenDigest(ly.Digest), ly.MediaType)
		mtLong := utils.ExpandMediaType(ly.MediaType)
		switch mtLong {
		case "application/vnd.docker.image.rootfs.diff.tar.gzip":
			mt = v1.MediaTypeImageLayerGzip
		case "application/vnd.docker.image.rootfs.diff.tar":
			mt = v1.MediaTypeImageLayer
		case v1.MediaTypeImageLayerGzip:
			mt = v1.MediaTypeImageLayerGzip
		case v1.MediaTypeImageLayer:
			mt = v1.MediaTypeImageLayer

		default:
			return res, utils.NewError(l, span, "unknown media type: %s", mtLong)
		}
		d := v1.Descriptor{
			MediaType: mt,
			Digest:    digest.Digest(ly.Digest),
			Size:      ly.Size,
		}
		// Put Layer in Graph
		lID, err := mag.Database.LayerDescriptorGetInsert(ctx, d)
		if err != nil {
			logrus.Error(err)
			return res, err
		}
		ld := types.LayerDesc{
			LayerID:    lID,
			Descriptor: d,
		}
		res = append(res, ld)
	}
	return
}

func (mag Magazine) LayersDeleteUnused(ctx context.Context) (err error) {
	_, span := tracer.Start(ctx, "LayersDeleteUnused")
	defer span.End()
	err = mag.Database.LayersDeleteUnused(ctx)
	if err != nil {
		err = errors.Wrapf(err, "failed to delete unused layers")
		return
	}
	err = mag.Database.BlobUploadDeleteUnused(ctx)
	if err != nil {
		err = errors.Wrapf(err, "failed to delete unused blobUploads")
		return
	}
	return
}

func (mag Magazine) BaseLayerEvalOS(ctx context.Context, lyr types.Layer) (os types.OS, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameLayer,
		"func":    "mag.BaseLayerEval",
		"layerID": lyr.ID,
	})
	defer span.End()
	_, e := mag.Minio.BlobStatObj(ctx, lyr.Digest)
	if e != nil {
		utils.ErrorHandler(l, span, e, "failed to stat layer")
		up, upFound, e := mag.Database.LayerGetUpstream(ctx, lyr.ID)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to get LayerGetUpstreamr")
			return
		}
		if !upFound {
			err = utils.NewError(l, span, "failed to find upstream layer")
			return
		}
		upt := upstream.UpstreamTriple{
			Reg:    up.Reg,
			Repo:   up.Repo,
			Tag:    up.Tag,
			Digest: up.Digest,
		}
		e = mag.PullLayer(ctx, upt)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to pull layer")
			return
		}
	}
	// Stream the layer from MinIO without buffering the full decompressed content.
	mt := utils.ExpandMediaType(lyr.MediaType)
	switch mt {
	case "application/vnd.docker.image.rootfs.diff.tar.gzip",
		"application/vnd.oci.image.layer.v1.tar+gzip":
		os, e = mag.EvalOSFromBlob(ctx, lyr.Digest)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to eval layer")
			return
		}
		return os, nil
	default:
		err = utils.NewError(l, span, "unknown media type: %s", mt)
		return
	}
}

// EvalOSFromBlob extracts OS information from the gzip-compressed tar layer
// stored in MinIO under dig using two sequential streaming reads — no full
// decompressed layer is ever buffered in memory. Pass 1 collects symlink
// headers only; pass 2 reads the single target file. This prevents OOM during
// pull-through cache ingest of images where /etc/os-release is a symlink
// (e.g. Alpine >=3.21 → usr/lib/os-release).
func (mag Magazine) EvalOSFromBlob(ctx context.Context, dig string) (os types.OS, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameLayer,
		"func":    "EvalOSFromBlob",
		"digest":  utils.ShortenDigest(dig),
	})
	defer span.End()

	const osRelease = "etc/os-release"

	// Pass 1: collect symlink/hardlink entries for etc/os-release (headers only,
	// no decompressed data buffered).
	r1, e := mag.Minio.BlobGetReader(ctx, dig)
	if e != nil {
		return os, utils.ErrorHandler(l, span, e, "failed to open blob reader (pass 1)")
	}
	links, e := scanGzipTarLinks(r1, osRelease)
	r1.Close()
	if e != nil {
		return os, utils.ErrorHandler(l, span, e, "failed to scan tar links")
	}

	// Build the set of names to read in pass 2: the direct name plus any targets.
	wanted := map[string]bool{osRelease: true}
	for _, t := range links {
		wanted[t] = true
	}

	// Pass 2: read only the identified file(s).
	r2, e := mag.Minio.BlobGetReader(ctx, dig)
	if e != nil {
		return os, utils.ErrorHandler(l, span, e, "failed to open blob reader (pass 2)")
	}
	contents, e := readGzipTarFiles(r2, wanted)
	r2.Close()
	if e != nil {
		return os, utils.ErrorHandler(l, span, e, "failed to read tar files")
	}

	content, found := resolveOSReleaseContent(osRelease, links, contents)
	if !found {
		return os, utils.NewError(l, span, "etc/os-release not found in layer %s", utils.ShortenDigest(dig))
	}
	return types.ParseOS(content)
}

// newTarReaderFromStream wraps r in a tar.Reader, auto-detecting gzip vs plain
// tar by peeking the first two bytes. The returned closer must be closed after
// use (it closes the gzip reader if one was opened; for plain tar it is a
// no-op). This allows EvalOSFromBlob to handle both real gzip layer blobs from
// production and plain-tar fixtures used in tests.
func newTarReaderFromStream(r io.Reader) (*tar.Reader, io.Closer, error) {
	br := bufio.NewReader(r)
	magic, e := br.Peek(2)
	if e != nil && e != io.EOF {
		return nil, nil, e
	}
	if len(magic) >= 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		gz, e := gzip.NewReader(br)
		if e != nil {
			return nil, nil, e
		}
		return tar.NewReader(gz), gz, nil
	}
	return tar.NewReader(br), io.NopCloser(br), nil
}

// scanGzipTarLinks walks the (gzip-compressed or plain) tar stream r and
// returns all symlink/hardlink entries whose normalized name equals wantedName,
// mapped to their resolved (root-stripped) link target.
func scanGzipTarLinks(r io.Reader, wantedName string) (map[string]string, error) {
	tr, closer, e := newTarReaderFromStream(r)
	if e != nil {
		return nil, e
	}
	defer closer.Close()
	links := map[string]string{}
	for {
		hdr, e := tr.Next()
		if e == io.EOF {
			return links, nil
		}
		if e != nil {
			return nil, e
		}
		if hdr.Typeflag != tar.TypeSymlink && hdr.Typeflag != tar.TypeLink {
			continue
		}
		name := strings.TrimPrefix(hdr.Name, "/")
		if name != wantedName {
			continue
		}
		target := hdr.Linkname
		if hdr.Typeflag == tar.TypeSymlink && !strings.HasPrefix(target, "/") {
			target = filepath.Join(filepath.Dir(name), target)
		}
		links[name] = strings.TrimPrefix(target, "/")
	}
}

// readGzipTarFiles extracts the content of regular-file entries whose
// normalized name appears in wanted from the (gzip-compressed or plain) tar
// stream r.
func readGzipTarFiles(r io.Reader, wanted map[string]bool) (map[string][]byte, error) {
	tr, closer, e := newTarReaderFromStream(r)
	if e != nil {
		return nil, e
	}
	defer closer.Close()
	contents := make(map[string][]byte, len(wanted))
	for {
		hdr, e := tr.Next()
		if e == io.EOF {
			return contents, nil
		}
		if e != nil {
			return nil, e
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		name := strings.TrimPrefix(hdr.Name, "/")
		if !wanted[name] {
			continue
		}
		data, e := io.ReadAll(tr)
		if e != nil {
			return nil, e
		}
		contents[name] = data
		if len(contents) == len(wanted) {
			return contents, nil
		}
	}
}

// resolveOSReleaseContent follows the symlink chain from name through links,
// returning the first matching content entry (up to 8 hops).
func resolveOSReleaseContent(name string, links map[string]string, contents map[string][]byte) (string, bool) {
	cur := name
	for i := 0; i < 8; i++ {
		if data, ok := contents[cur]; ok {
			return string(data), true
		}
		next, ok := links[cur]
		if !ok {
			break
		}
		cur = next
	}
	return "", false
}
