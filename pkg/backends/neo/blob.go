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

func (n Neo) CreateBlobFromChunk(ctx context.Context, bu *types.OCIBlobUpload, chunk types.BlobChunk) (err error) {
	l, _, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameBlob,
		"func":    "CreateBlobFromChunk",
		"repo":    bu.Repo,
		"digest":  utils.ShortenDigest(bu.Digest),
	})
	defer span.End()
	// Create Blob node with same digest as chunk
	dig := chunk.Digest
	if dig == "" {
		dig = bu.Digest
	}
	// MERGE (not CREATE) on the content digest: a :Blob is content-addressed, so
	// re-pushing identical content (e.g. the same model-layer image seeded under a
	// second profile) MUST reuse the existing node, not mint a duplicate. Duplicate
	// :Blob nodes with the same digest fan out `MATCH (l:Blob) WHERE l.digest=…`
	// during compose (ManifestLinkToLayer), producing extra HAS_LAYER edges → a
	// manifest whose layer count exceeds its config DiffIDs, which docker rejects
	// with "number of layers and diffIDs don't match".
	q := fmt.Sprintf(`MERGE (b:Blob {digest: "%s"}) ON CREATE SET b.label = "%s", b.created_at = TIMESTAMP(), b.size = %d`, dig, utils.ShortenDigest(dig), chunk.Bytes)
	q = q + ` RETURN ID(b) AS blobID`
	utils.Debug(l, span, q)
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
	return nil
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
		attribute.String(attrLogSeverity, attrSevDebug),
		attribute.String(attrLogQuery, q),
	))
	res, found, e := n.QueryOne(q)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, FailQueryExec)
		return false, err
	}
	if !found {
		msg := "blob not found"
		span.AddEvent("log", trace.WithAttributes(
			attribute.String(attrLogSeverity, attrSevDebug),
			attribute.String(attrLogMessage, msg),
		))
		l.Debug(msg)
		return false, nil
	}
	_, err = n.BlobFromResult(ctx, res)
	if err != nil {
		msg := "failed to create Blob from result"
		err = errors.Wrap(err, msg)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String(attrLogSeverity, attrSevError),
			attribute.String(attrLogMessage, msg),
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
		logrus.Errorf(errFailedRunQueryFmt, err)
		return
	}
	if result.Next() {
		found = true
		v, ok := result.Record().Get("ID")
		if !ok {
			err = fmt.Errorf("BlobGetByDigest: failed to get ID")
			return
		}
		utils.Debug(l, span, "BlobGetByDigest: got blob ID: %v", v)
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

func (n Neo) BlobDeleteByDigest(ctx context.Context, dig string) (err error) {
	l, _, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameBlob,
		"func":    "BlobDeleteByDigest",
		"digest":  utils.ShortenDigest(dig),
	})
	defer span.End()
	_ = l
	// b, found, err := n.BlobGetByDigest(ctx, dig)
	// if err != nil {
	// 	return utils.ErrorHandler(l, span, err, "failed to get blob by digest")
	// }
	// if !found {
	// 	err = utils.NewError(l, span, "BlobDeleteByDigest: blob with digest %s not found", dig)
	// 	return err
	// }
	q := `MATCH (n:Blob) WHERE n.digest = "%s" DETACH DELETE n RETURN id(n) AS deletedID`
	query := fmt.Sprintf(q, dig)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	res, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf(errFailedRunQueryFmt, err)
		return
	}
	if res.Next() {
		delIdInt := res.Record().Values[0]
		delId, ok := delIdInt.(int64)
		if !ok {
			err = utils.NewError(l, span, "Failed to convert deletedID to int64: %v", delIdInt)
			return err
		}
		utils.Debug(l, span, "Deleted Blob ID: %v", delId)
	}
	return err
}

// BlobUpdateSizeByDigest sets the size of a Blob node identified by its digest,
// but only when the current size is negative (placeholder). This prevents
// overwriting a previously correct value.
func (n Neo) BlobUpdateSizeByDigest(ctx context.Context, digest string, size int64) error {
	q := fmt.Sprintf(
		// Heal any blob node whose recorded size is below the authoritative
		// descriptor size — this covers both the "unknown" sentinel (size < 0)
		// and the size-0 placeholders minted by BlobEnsureByDigest when a client
		// deduped the upload ("Layer already exists"). Blob size is immutable per
		// digest, so a grow-only update (b.size < %[2]d) is safe and never shrinks
		// a real value; passing size 0 is a no-op.
		`MATCH (b:Blob {digest: "%[1]s"}) WHERE b.size < %[2]d SET b.size = %[2]d RETURN id(b) AS blobID`,
		digest, size,
	)
	_, err := n.Execute(q)
	return err
}

// BlobEnsureByDigest content-addressed-materializes a bare :Blob node for `dig`
// (size only) when it is missing from the graph but its bytes survive in the
// object store. This is the graph half of the content-addressed HEAD fallback: a
// client that HEADs an already-present object is answered 200 and skips the
// upload, so no :Blob node is (re)created — but the manifest-link path
// (ManifestLinkToArtifactConfig / ManifestLinkToConfig, which MATCH (b:Blob
// {digest})) needs the node to exist or it 400s "config blob not found". MERGE
// on the content digest keeps it idempotent and content-addressed (no duplicate
// :Blob nodes, cf. CreateBlobFromChunk).
func (n Neo) BlobEnsureByDigest(ctx context.Context, dig string, size int64) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameBlob,
		"func":    "BlobEnsureByDigest",
		"digest":  utils.ShortenDigest(dig),
	})
	defer span.End()
	_ = ctx
	q := fmt.Sprintf(`MERGE (b:Blob {digest: "%s"}) ON CREATE SET b.label = "%s", b.created_at = TIMESTAMP(), b.size = %d`, dig, utils.ShortenDigest(dig), size)
	utils.Debug(l, span, q)
	if _, err = n.Execute(q); err != nil {
		return utils.ErrorHandler(l, span, err, FailQueryExec)
	}
	return nil
}
