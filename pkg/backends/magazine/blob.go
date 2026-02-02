package magazine

import (
	"context"
	"fmt"
	"net/http"

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
		"package": "magazine/blob.go",
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
		// multiple chunks uploaded, need to assemble
		for _, c := range chunks {
			utils.NewError(l, span, "chunk: ID:%d Start:%d Bytes:%d > %s", c.ID, c.StartByte, c.Bytes, utils.ShortenDigest(c.Digest))
		}

	}
	return nil
}

func (mag Magazine) BlobGetByHTTPRequest(ctx context.Context, r *http.Request) (blob *types.Blob, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": "magazine/blob.go",
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
