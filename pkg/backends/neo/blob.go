package neo

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	pkgNameBlob = "neo/blob.go"
)

// CreateBlobFromChunk(ctx context.Context, bu *types.OCIBlobUpload, chunk types.BlobChunk) (err error)
func (n Neo) CreateBlobFromChunk(ctx context.Context, bu *types.OCIBlobUpload, chunk types.BlobChunk) (err error) {
	_, span := tracer.Start(ctx, "CreateBlobFromChunk")
	defer span.End()
	// Create Blob node with same digest as chunk
	q := fmt.Sprintf(`CREATE (b:Blob {digest: "%s", label: "%s", created_at: TIMESTAMP(), size: %d})`, bu.Digest, utils.ShortenDigest(bu.Digest), chunk.Bytes)
	q = q + ` RETURN ID(b) AS blobID`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("query", q),
	))
	res, err := n.Execute(q)
	if err != nil {
		span.RecordError(err)
		return
	}
	blobID, err := res.GetInt64("blobID")
	if err != nil {
		span.RecordError(err)
		return
	}
	// Link BlobUpload to Blob
	q = fmt.Sprintf(`MATCH (bu:BlobUpload {uuid: "%s"}), (b:Blob)`, bu.UUID.String())
	q += fmt.Sprintf(` WHERE ID(b) = %d`, blobID)
	q += ` CREATE (bu)-[:COMPLETED_UPLOAD]->(b)`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("query", q),
	))
	_, err = n.Execute(q)
	if err != nil {
		span.RecordError(err)
		return
	}
	return
}

func (n Neo) BlobExists(ctx context.Context, up upstream.UpstreamTriple) (found bool, err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameBlob,
		"func":    "BlobExists",
		"repo":    up.Repo,
		"tag":     up.Tag,
		"digest":  utils.ShortenDigest(up.Digest.String()),
	})
	ctx, span := tracer.Start(ctx, "neo.BlobExists")
	_ = ctx
	defer span.End()
	span.SetAttributes(
		attribute.Key("repo").String(up.Repo),
		attribute.Key("tag").String(up.Tag),
		attribute.Key("digest").String(utils.ShortenDigest(up.Digest.String())),
	)
	q := fmt.Sprintf(`MATCH (b:Blob) WHERE b.digest = "%s"`, up.Digest.String())
	q += ` RETURN id(b) AS blobID, b.digest AS blobDigest, b.size AS blobSize`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	res, found, e := n.QueryOne(q)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailQueryExec)
		return false, err
	}
	if !found {
		msg := "blob not found"
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.message", msg),
		))
		l.Debug(msg)
		return false, nil
	}
	_, err = n.BlobFromResult(ctx, res)
	if err != nil {
		msg := "failed to create Blob from result"
		err = errors.Wrap(err, msg)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "error"),
			attribute.String("log.message", msg),
		))
		span.RecordError(err)
		l.Error(msg)
		return false, err
	}
	return true, nil
}

func (n Neo) BlobFromResult(ctx context.Context, res types.Result) (b types.Blob, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameBlob,
		"func":    "BlobFromResult",
	})
	defer span.End()
	_ = l
	_ = ctx
	e := res.CheckKeys("blobID", "blobDigest", "blobSize")
	if e != nil {
		err = utils.ErrorHandler(l, span, e, "failed to check keys")
		return
	}
	id, _ := res.GetInt64("blobID")
	size, _ := res.GetInt64("blobSize")
	dig, _ := res.GetStr("blobDigest")
	b = types.Blob{
		ID:     id,
		Digest: dig,
		Size:   size,
	}
	return
}

func (n Neo) BlobGetByDigest(ctx context.Context, dig string) (b *types.Blob, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameBlob,
		"func":    "BlobGetByDigest",
		"digest":  utils.ShortenDigest(dig),
	})
	_ = l
	_ = ctx
	b = &types.Blob{}
	defer span.End()
	q := `MATCH (n:Blob) WHERE n.digest = "%s"`
	q += ` RETURN id(n) AS ID, n.digest AS digest, n.size AS size`
	query := fmt.Sprintf(q, dig)
	span.AddEvent("neo", trace.WithAttributes(attribute.String("query", query)))
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("Failed to run query: %s", err)
		return
	}
	if result.Next() {
		found = true
		v, ok := result.Record().Get("ID")
		if !ok {
			err = fmt.Errorf("BlobGetByDigest: failed to get ID")
			return
		}
		logrus.Infof("BlobGetByDigest: got blob ID: %v", v)
		b.ID = v.(int64)
		v, ok = result.Record().Get("size")
		if !ok {
			err = fmt.Errorf("BlobGetByDigest: failed to get size")
			return
		}
		b.Size = v.(int64)
		v, ok = result.Record().Get("digest")
		if !ok {
			err = fmt.Errorf("BlobGetByDigest: failed to get digest")
			return
		}
		b.Digest = v.(string)
	}
	return
}
