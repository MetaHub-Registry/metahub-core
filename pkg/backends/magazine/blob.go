package magazine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	pkgNameBlob        = "magazine/blob.go"
	failLinkBlobUpload = "failed to link layer to blob upload"
	// msgSkippedS3Upload is logged (and shared by every store path) whenever the
	// content-addressed dedup fires — the object was already in the bucket so the
	// S3 PutObject egress was skipped.
	msgSkippedS3Upload = "blob %s already in object store (%d bytes) — skipped S3 upload"
	// failCreateComposedBlob is the error wrap used wherever a composed :Blob node
	// is (re)created from chunks — shared by the dedup-skip and stream-compose paths.
	failCreateComposedBlob = "failed to create composed blob node"
)

var (
	ErrJSONNOTCONFIG = fmt.Errorf("buf is JSON, but not v1.Image")
)

// dedupMissReason classifies why the content-addressed dedup skip did not fire,
// for the perf-debug instrumentation in StoreBlobUploadReader.
func dedupMissReason(statFound bool) string {
	if !statFound {
		return "object-absent"
	}
	return "size-mismatch"
}

// blobObjPresent reports whether the content-addressed object `dig` is already in
// the bucket with the expected `size`, so the S3 PutObject can be skipped. Blob
// objects are keyed by digest, so a size match on an existing object is a safe
// content-addressed dedup (sha256 collisions aside). It records the dedup
// decision on `span` (and logs it) so every store path — buffered PATCH chunk
// (StoreBlobUpload), streamed monolithic PUT (StoreBlobUploadReader), and
// multi-chunk compose (ComposeFromChunks) — shares one instrumented decision
// point in traces/VictoriaLogs. `size` is the expected object size (len(buf) for
// buffered, the declared Content-Length for streamed, the summed chunk length
// for compose). The returned `statFound` distinguishes "object absent" from
// "size mismatch" for the miss-path reason string.
func (mag Magazine) blobObjPresent(ctx context.Context, l *logrus.Entry, span trace.Span, dig string, size int64) (skip, statFound bool) {
	obj, statErr := mag.Minio.BlobStatObj(ctx, dig)
	statFound = statErr == nil
	sizeMatch := statFound && obj.Size == size
	statErrStr := ""
	if statErr != nil {
		statErrStr = statErr.Error()
	}
	span.SetAttributes(
		attribute.String("blob.digest", dig),
		attribute.Int64("blob.declared_size", size),
		attribute.Bool("blob.stat_found", statFound),
		attribute.Int64("blob.stat_size", obj.Size),
		attribute.Bool("blob.size_match", sizeMatch),
		attribute.Bool("blob.dedup_skip", sizeMatch),
		attribute.String("blob.stat_err", statErrStr),
	)
	utils.Info(l, span, "blob-dedup-decision digest=%s declared_size=%d stat_found=%v stat_size=%d size_match=%v skip=%v stat_err=%q",
		dig, size, statFound, obj.Size, sizeMatch, sizeMatch, statErrStr)
	return sizeMatch, statFound
}

// drainAndLinkPresent handles the content-addressed dedup HIT: the object is
// already in the bucket, so drain+verify the request body (integrity check, fast
// LAN) without re-uploading, then link the blob upload to the existing object.
func (mag Magazine) drainAndLinkPresent(ctx context.Context, bu *types.OCIBlobUpload, r io.Reader, size int64) error {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameBlob,
		"func":    "drainAndLinkPresent",
		"digest":  bu.Digest,
	})
	defer span.End()
	digester := digest.SHA256.Digester()
	if _, ce := io.Copy(digester.Hash(), r); ce != nil {
		return utils.ErrorHandler(l, span, ce, "failed to drain body for already-present blob")
	}
	if got := digester.Digest().String(); got != bu.Digest {
		return utils.NewError(l, span, "streamed digest %s does not match expected %s (object already present)", got, bu.Digest)
	}
	utils.Info(l, span, msgSkippedS3Upload, utils.ShortenDigest(bu.Digest), size)
	if e := mag.Database.BlobUploadUpdateByUUID(ctx, bu); e != nil {
		return utils.ErrorHandler(l, span, e, failLinkBlobUpload)
	}
	return nil
}

// linkComposedPresent records the :Blob node for an already-present composed
// object (content-addressed dedup HIT in ComposeFromChunks) without re-streaming
// the chunks back through the pod and re-uploading the assembled blob to S3. Kept
// as a helper so the multi-chunk compose path stays under the cognitive-complexity
// limit (cf. drainAndLinkPresent for the streamed PUT path).
func (mag Magazine) linkComposedPresent(ctx context.Context, l *logrus.Entry, span trace.Span, bu *types.OCIBlobUpload, total int64) error {
	utils.Info(l, span, msgSkippedS3Upload, utils.ShortenDigest(bu.Digest), total)
	if e := mag.Database.CreateBlobFromChunk(ctx, bu, types.BlobChunk{Digest: bu.Digest, Bytes: total}); e != nil {
		return utils.ErrorHandler(l, span, e, failCreateComposedBlob)
	}
	return nil
}

func (mag Magazine) StoreBlobUpload(ctx context.Context, bu *types.OCIBlobUpload, buf []byte) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":  pkgNameBlob,
		"func":     "StoreBlobUpload",
		"blobType": bu.BlobType,
		"digest":   bu.Digest,
	})
	defer span.End()
	// Content-addressed dedup (mirrors StoreBlobUploadReader): blob objects are
	// keyed by digest, so if the object is already in the bucket there is nothing
	// to upload. The buffered PATCH-chunk path (BlobUploadPatchHandler ->
	// StoreBlobUpload -> PutBlob) previously re-wrote every chunk to S3
	// unconditionally — traces showed min.BlobPutObj firing on every PATCH (avg
	// ~0.9s, up to ~1.9s each) — so a re-seed after the e2e --empty-graph wipe
	// re-uploaded all chunks over the slow remote-S3 egress for bytes already in
	// the bucket. The buffered payload already hashes to bu.Digest (the caller set
	// it from the body), so a size match is a safe content-addressed skip — no
	// re-hash needed.
	if skip, _ := mag.blobObjPresent(ctx, l, span, bu.Digest, int64(len(buf))); skip {
		utils.Info(l, span, msgSkippedS3Upload, utils.ShortenDigest(bu.Digest), len(buf))
	} else {
		utils.Debug(l, span, "uploading to MinIO: %s", utils.ShortenDigest(bu.Digest))
		if err = mag.Minio.PutBlob(ctx, buf, bu.Digest); err != nil {
			return utils.ErrorHandler(l, span, err, "failed to put layer into minio")
		}
	}
	utils.Debug(l, span, "Upload done, update database")
	// Link Layer to BlobUpload
	if e := mag.Database.BlobUploadUpdateByUUID(ctx, bu); e != nil {
		return utils.ErrorHandler(l, span, e, failLinkBlobUpload)
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
	// INSTRUMENTATION (perf-debug, non-behavioral): the shared helper hoists the
	// object stat so the exact dedup decision is visible per blob PUT in
	// VictoriaLogs/traces — whether the content-addressed object was already in the
	// bucket (skip) or streamed again (redundant re-upload of ~1GB), and the stat
	// result that drove it. This settles "object deleted between runs" vs "dedup
	// never fires" without guessing.
	skip, statFound := mag.blobObjPresent(ctx, l, span, bu.Digest, size)
	if skip {
		// dedup HIT: the content-addressed object is already in the bucket — skip
		// the S3 upload, drain+verify the body, and link it (helper keeps this
		// method under the cognitive-complexity limit).
		return mag.drainAndLinkPresent(ctx, bu, r, size)
	}
	// Redundant re-upload path: the dedup skip did NOT fire. Log loudly with the
	// reason (object absent vs size mismatch) and time the S3 write so the run's
	// logs quantify exactly which ~1GB PUT(s) hit the slow S3 egress.
	uploadStart := time.Now()
	utils.Info(l, span, "streaming to MinIO (dedup MISS, redundant if already present): digest=%s size=%d reason=%s",
		bu.Digest, size, dedupMissReason(statFound))
	digester := digest.SHA256.Digester()
	tee := io.TeeReader(r, digester.Hash())
	if e := mag.Minio.BlobPutReader(ctx, tee, size, bu.Digest); e != nil {
		return utils.ErrorHandler(l, span, e, "failed to stream layer into minio")
	}
	span.SetAttributes(attribute.Float64("blob.upload_seconds", time.Since(uploadStart).Seconds()))
	utils.Info(l, span, "streamed-to-MinIO digest=%s size=%d upload_seconds=%.1f", bu.Digest, size, time.Since(uploadStart).Seconds())
	if got := digester.Digest().String(); got != bu.Digest {
		_ = mag.Minio.BlobRemoveObj(ctx, bu.Digest)
		return utils.NewError(l, span, "uploaded digest %s does not match expected %s", got, bu.Digest)
	}
	utils.Debug(l, span, "Upload done, update database")
	if e := mag.Database.BlobUploadUpdateByUUID(ctx, bu); e != nil {
		return utils.ErrorHandler(l, span, e, failLinkBlobUpload)
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
	// INSTRUMENTATION (perf-debug): make blob deletions loud so a run's logs reveal
	// any code path that removes a (large, content-addressed) blob between seeds.
	utils.Info(l, span, "BlobDelete CALLED repo=%s tag=%s digest=%s", req.Repo, req.Tag, req.Digest)
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
		if e := mag.Database.CreateBlobFromChunk(ctx, bu, chunks[0]); e != nil {
			return utils.ErrorHandler(l, span, e, "failed to create blob from chunk")
		}
		return nil
	default:
		return mag.composeMultiChunk(ctx, l, span, bu, chunks)
	}
}

// composeMultiChunk assembles a multi-chunk upload into the single whole-blob
// object. Each chunk was stored in MinIO under its own content digest during
// PATCH; they are streamed back in start_byte order through a sha256 TeeReader
// and stream-written under the whole-blob digest (bu.Digest) — never buffering
// the full payload (models can be 100 GB). Split out of ComposeFromChunks to keep
// that method under the cognitive-complexity limit.
func (mag Magazine) composeMultiChunk(ctx context.Context, l *logrus.Entry, span trace.Span, bu *types.OCIBlobUpload, chunks []types.BlobChunk) error {
	sort.Slice(chunks, func(i, j int) bool { return chunks[i].StartByte < chunks[j].StartByte })
	var total int64
	for _, c := range chunks {
		total += c.Bytes - c.StartByte + 1 // Bytes is the inclusive END offset
	}
	// Content-addressed dedup: the assembled whole-blob object is keyed by
	// bu.Digest. If it is already in the bucket (matching total size), skip the
	// re-compose+re-upload entirely (reading every chunk back through the pod +
	// re-streaming to S3) and just (re)create the :Blob node — the same
	// wipe-and-reseed scenario handled in StoreBlobUpload{,Reader}.
	if bu.Digest != "" {
		if skip, _ := mag.blobObjPresent(ctx, l, span, bu.Digest, total); skip {
			return mag.linkComposedPresent(ctx, l, span, bu, total)
		}
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
		return utils.ErrorHandler(l, span, e, failCreateComposedBlob)
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
