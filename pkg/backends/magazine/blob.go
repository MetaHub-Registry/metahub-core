package magazine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"

	"github.com/opencontainers/go-digest"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgNameBlob = "magazine/blob.go"
)

var (
	ErrJSONNOTCONFIG = fmt.Errorf("buf is JSON, but not v1.Image")
)

func (mag Magazine) StoreBlobUpload(ctx context.Context, bu *types.OCIBlobUpload, buf []byte) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":  pkgNameBlob,
		"func":     "StoreBlobUpload",
		"blobType": bu.BlobType,
		"digest":   bu.Digest,
	})
	defer span.End()
	utils.Debug(l, span, "uploading to MinIO: %s", utils.ShortenDigest(bu.Digest))
	err = mag.Minio.PutBlob(ctx, buf, bu.Digest)
	if err != nil {
		return utils.ErrorHandler(l, span, err, "failed to put layer into minio")
	}
	utils.Debug(l, span, "Upload done, update database")
	// Link Layer to BlobUpload
	if e := mag.Database.BlobUploadUpdateByUUID(ctx, bu); e != nil {
		return utils.ErrorHandler(l, span, e, "failed to link layer to blob upload")
	}
	return nil
	//// Are we dealing with a Config or a layer?
	// DEPRECATED: blobs are blobs now - no need to differentiate here
	// v1Img := v1.Image{}
	// err = json.Unmarshal(buf, &v1Img)
	// switch {
	// case v1Img.Architecture != "":
	// 	l.Debug(l, span, "Blob is a config")
	// 	return mag.StoreConfigUpload(ctx, bu, buf)
	// case err != nil && strings.HasPrefix(err.Error(), "invalid character"):
	// 	l.Debug(l, span, "Blob is a layer")
	// 	return mag.StoreLayerUpload(ctx, bu, buf)
	// default:
	// 	l.Debug(l, span, "Blob is neither a layer nor a config")
	// 	return ErrJSONNOTCONFIG
	// }
}

// StoreBlobUploadReader streams the blob body from r (exactly size bytes) into
// MinIO under bu.Digest via a sha256 TeeReader — never buffering the whole
// payload, so a monolithic model-layer PUT (10-100 GB) does not OOM the process
// the way io.ReadAll + StoreBlobUpload([]byte) does. bu.Digest must hold the
// expected whole-blob digest (from the monolithic PUT's ?digest=); the streamed
// content is verified against it and removed on mismatch.
func (mag Magazine) StoreBlobUploadReader(ctx context.Context, bu *types.OCIBlobUpload, r io.Reader, size int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":  pkgNameBlob,
		"func":     "StoreBlobUploadReader",
		"blobType": bu.BlobType,
		"digest":   bu.Digest,
	})
	defer span.End()
	if bu.Digest == "" {
		return utils.NewError(l, span, "StoreBlobUploadReader requires bu.Digest (expected blob digest)")
	}
	// Content-addressed dedup: blob objects are keyed by digest, so if the object
	// is already in the bucket there is nothing to upload. This is the common case
	// after the e2e conformance graph wipe (parse rules --empty-graph=keep-upstream
	// DETACH DELETEs the :Blob nodes but leaves the S3 objects intact), where a
	// re-seed would otherwise re-stream ~1GB model layers over the slow remote-S3
	// egress. Drain+verify the body through the same sha256 so integrity is still
	// checked (fast, LAN), then skip only the PutObject; the caller still creates
	// the :Blob node from the already-present object via ComposeFromChunks.
	if obj, e := mag.Minio.BlobStatObj(ctx, bu.Digest); e == nil && obj.Size == size {
		digester := digest.SHA256.Digester()
		if _, ce := io.Copy(digester.Hash(), r); ce != nil {
			return utils.ErrorHandler(l, span, ce, "failed to drain body for already-present blob")
		}
		if got := digester.Digest().String(); got != bu.Digest {
			return utils.NewError(l, span, "streamed digest %s does not match expected %s (object already present)", got, bu.Digest)
		}
		utils.Info(l, span, "blob %s already in object store (%d bytes) — skipped S3 upload", utils.ShortenDigest(bu.Digest), size)
		if e := mag.Database.BlobUploadUpdateByUUID(ctx, bu); e != nil {
			return utils.ErrorHandler(l, span, e, "failed to link layer to blob upload")
		}
		return nil
	}
	utils.Debug(l, span, "streaming to MinIO: %s (%d bytes)", utils.ShortenDigest(bu.Digest), size)
	digester := digest.SHA256.Digester()
	tee := io.TeeReader(r, digester.Hash())
	if e := mag.Minio.BlobPutReader(ctx, tee, size, bu.Digest); e != nil {
		return utils.ErrorHandler(l, span, e, "failed to stream layer into minio")
	}
	if got := digester.Digest().String(); got != bu.Digest {
		_ = mag.Minio.BlobRemoveObj(ctx, bu.Digest)
		return utils.NewError(l, span, "uploaded digest %s does not match expected %s", got, bu.Digest)
	}
	utils.Debug(l, span, "Upload done, update database")
	if e := mag.Database.BlobUploadUpdateByUUID(ctx, bu); e != nil {
		return utils.ErrorHandler(l, span, e, "failed to link layer to blob upload")
	}
	return nil
}

func (mag Magazine) BlobDelete(ctx context.Context, req types.Request) (found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameBlob,
		"func":    "BlobDelete",
		"repo":    req.Repo,
		"tag":     req.Tag,
		"digest":  utils.ShortenDigest(req.Digest),
	})
	defer span.End()
	_, found, err = mag.Database.BlobGetByDigest(ctx, req.Digest)
	if err != nil {
		return false, utils.ErrorHandler(l, span, err, types.FailBlobGetByDigest)
	}
	if found {
		if err = mag.Database.BlobDeleteByDigest(ctx, req.Digest); err != nil {
			return false, utils.ErrorHandler(l, span, err, types.FailBlobGetByDigest)
		}
		// Remove Blob
		if err = mag.Minio.BlobRemoveObj(ctx, req.Digest); err != nil {
			return false, utils.ErrorHandler(l, span, err, types.FailLayerRemoveObj)
		}
		return true, nil
	}
	// Not a layer, so it must be a config
	_, found, err = mag.Database.ConfigGetByDigest(ctx, req.Digest)
	if err != nil {
		return false, err
	}
	if found {
		if err = mag.Database.ConfigDeleteByDigest(ctx, req.Digest); err != nil {
			return false, utils.ErrorHandler(l, span, err, types.FailConfigDeleteByDigest)
		}
		// Remove Blob
		if err = mag.Minio.BlobRemoveObj(ctx, req.Digest); err != nil {
			return false, utils.ErrorHandler(l, span, err, types.FailConfigRemoveObj)
		}
		return true, nil
	}
	return false, nil
}

func (mag Magazine) BlobExists(ctx context.Context, up upstream.UpstreamTriple) (found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameBlob,
		"func":    "BlobExists",
		"repo":    up.Repo,
		"tag":     up.Tag,
		"digest":  utils.ShortenDigest(up.Digest.String()),
	})
	defer span.End()
	found, e := mag.Database.BlobExists(ctx, up)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to execute ConfigExists")
		return false, err
	}

	// Check if blob exists in Minio
	_, err = mag.Minio.BlobStatObj(ctx, up.Digest.String())
	if err != nil {
		// Layer does not exist in Minio, so we need to pull it through
		utils.Debug(l, span, "layer does not exist in Minio")
		e = mag.PullLayer(ctx, up)
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "failed to pull layer")
			return false, err
		}
		_, e = mag.Minio.BlobStatObj(ctx, up.Digest.String())
		if e != nil {
			err = utils.ErrorHandler(l, span, e, "after pulling layer the layer is still not in Minio")
			return false, err
		}
	}
	return found, nil
}

func (mag Magazine) ComposeFromChunks(ctx context.Context, bu *types.OCIBlobUpload, chunks []types.BlobChunk) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameBlob,
		"func":    "ComposeFromChunks",
	})
	defer span.End()
	switch len(chunks) {
	case 0:
		return fmt.Errorf("why did we upload a blob with no chunks?")
	case 1:
		utils.Debug(l, span, "single chunk upload, finalizing")
		chunk := chunks[0]
		e := mag.Database.CreateBlobFromChunk(ctx, bu, chunk)
		if e != nil {
			return utils.ErrorHandler(l, span, e, "failed to create blob from chunk")
		}
	default:
		// Each chunk was stored in MinIO under its own content digest during
		// PATCH. Stream them back in start_byte order through a sha256 TeeReader
		// and stream-write the assembled blob under the whole-blob digest
		// (bu.Digest) — never buffering the full payload (models can be 100 GB).
		sort.Slice(chunks, func(i, j int) bool { return chunks[i].StartByte < chunks[j].StartByte })
		var total int64
		for _, c := range chunks {
			total += c.Bytes - c.StartByte + 1 // Bytes is the inclusive END offset
		}
		seq := &chunkSeqReader{ctx: ctx, mag: mag, chunks: chunks}
		defer seq.Close()
		digester := digest.SHA256.Digester()
		tee := io.TeeReader(seq, digester.Hash())
		if e := mag.Minio.BlobPutReader(ctx, tee, total, bu.Digest); e != nil {
			return utils.ErrorHandler(l, span, e, "failed to stream-compose blob %s", utils.ShortenDigest(bu.Digest))
		}
		if got := digester.Digest().String(); bu.Digest != "" && got != bu.Digest {
			_ = mag.Minio.BlobRemoveObj(ctx, bu.Digest)
			return utils.NewError(l, span, "composed digest %s does not match expected %s", got, bu.Digest)
		}
		utils.Info(l, span, "composed %d chunks -> blob %s (%d bytes)", len(chunks), utils.ShortenDigest(bu.Digest), total)
		if e := mag.Database.CreateBlobFromChunk(ctx, bu, types.BlobChunk{Digest: bu.Digest, Bytes: total}); e != nil {
			return utils.ErrorHandler(l, span, e, "failed to create composed blob node")
		}
	}
	return nil
}

// chunkSeqReader streams a sequence of MinIO-stored chunks as one contiguous
// io.Reader, opening (and closing) exactly one chunk at a time so composing a
// many-thousand-chunk blob never holds more than a single chunk's connection.
type chunkSeqReader struct {
	ctx    context.Context
	mag    Magazine
	chunks []types.BlobChunk
	idx    int
	cur    io.ReadCloser
}

func (r *chunkSeqReader) Read(p []byte) (int, error) {
	for {
		if r.cur == nil {
			if r.idx >= len(r.chunks) {
				return 0, io.EOF
			}
			rc, err := r.mag.Minio.BlobGetReader(r.ctx, r.chunks[r.idx].Digest)
			if err != nil {
				return 0, err
			}
			r.cur = rc
		}
		n, err := r.cur.Read(p)
		if err == io.EOF {
			_ = r.cur.Close()
			r.cur = nil
			r.idx++
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, err
	}
}

func (r *chunkSeqReader) Close() error {
	if r.cur != nil {
		err := r.cur.Close()
		r.cur = nil
		return err
	}
	return nil
}

func (mag Magazine) BlobGetByHTTPRequest(ctx context.Context, r *http.Request) (blob *types.Blob, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameBlob,
		"func":    "BlobGetByHTTPRequest",
	})
	defer span.End()
	req := types.NewRequest(ctx, r)
	utils.Debug(l, span, "BlobGetByHTTPRequest: %s", req.String())
	// Fetch blob from Database
	blob, found, err = mag.Database.BlobGetByDigest(ctx, req.Digest)
	if err != nil {
		utils.ErrorHandler(l, span, err, "failed to fetch blob from database")
		return
	}
	if !found {
		utils.Debug(l, span, "did not find blob with dig: %s", req.Digest)
		return
	}
	if req.Method == "GET" {
		// Fetch Blob
		b, err := mag.Minio.BlobGetBytes(ctx, req.Digest)
		switch {
		case err != nil && err.Error() == "The specified key does not exist.":
			// found is false
			utils.Debug(l, span, "did not find blob data with dig: %s", req.Digest)
			return blob, false, nil
		case err != nil:
			utils.ErrorHandler(l, span, err, "failed to fetch blob from minio")
			return nil, false, err
		}
		utils.Debug(l, span, "fetched blob data with dig: %s", req.Digest)
		blob.Data = b
	}
	// No error - off we go
	found = true
	return
}
