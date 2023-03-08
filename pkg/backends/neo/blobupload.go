package neo

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/metahub-registry/metahub-core/pkg/tracing"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (n Neo) BlobUpload_Create_ByOCI(ctx context.Context, i *types.OCIBlobUpload) (err error) {
	_, span := tracer.Start(ctx, "BlobUpload_Create_ByOCI")
	defer span.End()
	i.UUID = uuid.New()
	span.SetAttributes(
		attribute.Key("uuid").String(i.UUID.String()),
	)
	i.Location = fmt.Sprintf("/v2/%s/blobs/uploads/%s", i.Repo, i.UUID.String())
	q := fmt.Sprintf(`CREATE (bu:BlobUpload {upload_user: "%s", upload_profile:"%s", digest: "%s", uuid: "%s", location:"%s", created_at: TIMESTAMP()})`, i.UploadUser, i.UploadProfile, i.Digest, i.UUID.String(), i.Location)
	q += ` RETURN ID(bu) AS blobuploadID`
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("query", q),
	))
	res, err := n.Execute(q)
	if err != nil {
		span.RecordError(err)
		return
	}
	i.ID, err = res.GetInt64("blobuploadID")
	return
}

func (n Neo) BlobUpload_GetOCI_ByUUID(ctx context.Context, uuid string) (bu types.OCIBlobUpload, found bool, err error) {
	_, span := tracer.Start(ctx, "BlobUpload_GetOCI_ByUUID")
	span.SetAttributes(
		attribute.Key("uuid").String(uuid),
	)
	defer span.End()
	q := `MATCH (bu:BlobUpload {uuid: "%s"})`
	q += ` RETURN ID(bu) AS blobuploadID, bu.upload_user AS blobuploadUploadUser, bu.upload_user AS blobuploadUploadProfile, bu.digest AS blobuploadDigest,`
	q += ` bu.uuid AS blobuploadUUID, bu.location AS blobuploadLocation`
	query := fmt.Sprintf(q, uuid)
	res, found, err := n.QueryOne(query)
	if err != nil || !found {
		return
	}
	bu, err = OCIBlobUploadFromResult(res)
	return
}

func (n Neo) BlobUpload_Update_ByUUID(ctx context.Context, i *types.OCIBlobUpload) (err error) {
	// TODO: Is digest a layer or a config?
	_, span := tracer.Start(ctx, "BlobUpload_Update_ByUUID")
	shortDig := utils.ShortenDigest(i.Digest)
	span.SetAttributes(
		attribute.Key("buID").Int64(i.ID),
		attribute.Key("digets").String(shortDig),
	)
	defer span.End()
	q := fmt.Sprintf(`MATCH (n) WHERE n.digest = "%s" RETURN LABELS(n) AS nodeType, ID(n) AS nodeID`, i.Digest)
	span.AddEvent("log", trace.WithAttributes(
		attribute.String("log.severity", "debug"),
		attribute.String("log.query", q),
	))
	res, found, err := n.QueryOne(q)
	if err != nil {
		return
	}
	if found {
		nodeID, err := res.GetInt64("nodeID")
		if err != nil {
			return err
		}
		tracing.SpanLog(span, "debug", "found node with id '%d' with digest '%s'", nodeID, shortDig)
		nodeType, err := res.GetStr("nodeType")
		if err != nil {
			return err
		}
		q = `MATCH (bu:BlobUpload) WHERE id(bu) = %d`
		q += ` MATCH (n:%s) WHERE id(n) = %d`
		q += ` MERGE (bu)-[:IS]->(n)`
		query := fmt.Sprintf(q, i.ID, nodeType, nodeID)
		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "debug"),
			attribute.String("log.query", query),
		))
		_, err = n.Execute(query)
		if err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("no node with digest '%s' found", i.Digest)
}

func (n Neo) BlobUpload_Complete(ctx context.Context, i *types.OCIBlobUpload) (err error) {
	_, span := tracer.Start(ctx, "UploadChunk_Create")
	defer span.End()
	span.SetAttributes(
		attribute.Key("id").Int64(i.ID),
	)
	q := `MATCH (bu:BlobUpload) WHERE id(bu) = %d`
	q += ` SET bu.finished_at = TIMESTAMP()`
	query := fmt.Sprintf(q, i.ID)
	span.AddEvent("query", trace.WithAttributes(
		attribute.String("query", query),
	))
	_, err = n.Execute(query)
	return
}

// // CHUNKS
func (n Neo) UploadChunk_Create(ctx context.Context, buID, startByte, lenBytes int64) (bc types.BlobChunks, err error) {
	_, span := tracer.Start(ctx, "UploadChunk_Create")
	defer span.End()
	span.SetAttributes(
		attribute.Key("id").Int64(buID),
		attribute.Key("startByte").Int64(startByte),
		attribute.Key("lenBytes").Int64(lenBytes),
	)
	bc = types.BlobChunks{
		BlobUploadID: buID,
		CreatedAt:    time.Now(),
		StartByte:    startByte,
		Bytes:        lenBytes,
	}
	q := `MATCH (bu:BlobUpload) WHERE id(bu) = %d`
	q += ` CREATE (bc:BlobChunk {start_byte: %d, bytes: %d, range: "%s", created_at: TIMESTAMP()})`
	q += ` MERGE (bu)-[:CHUNK]->(bc)`
	q += ` RETURN ID(bc) AS blobchunkID`
	query := fmt.Sprintf(q, buID, startByte, lenBytes, fmt.Sprintf("%d-%d", startByte, startByte+lenBytes))
	span.AddEvent("query", trace.WithAttributes(
		attribute.String("query", query),
	))
	res, err := n.Execute(query)
	if err != nil {
		return
	}
	bc.ID, err = res.GetInt64("blobchunkID")
	return
}

func (n Neo) UploadChunk_Get_ByUUID(ctx context.Context, buID int64) (bcs []types.BlobChunks, err error) {
	_, span := tracer.Start(ctx, "UploadChunk_Get_ByUUID")
	defer span.End()
	span.SetAttributes(
		attribute.Key("id").Int64(buID),
	)
	q := `MATCH (bu:BlobUpload) WHERE id(bu) = %d`
	q += ` MATCH (bu)-[:CHUNK]->(bc:BlobChunk)`
	q += ` RETURN ID(bc) AS blobchunkID, bc.start_byte AS blobchunkStartByte, bc.bytes AS blobchunkBytes, bc.range AS blobchunkRange, bc.created_at AS blobchunkCreatedAt`
	query := fmt.Sprintf(q, buID)
	span.AddEvent("query", trace.WithAttributes(
		attribute.String("query", query),
	))
	res, err := n.Query(query)
	if err != nil {
		span.RecordError(err)
		return
	}
	for _, r := range res {
		bc, err := BlobChunkFromResult(r)
		if err != nil {
			span.RecordError(err)
			return bcs, err
		}
		bcs = append(bcs, bc)
	}
	return
}

// / Helpers
func BlobChunkFromResult(res types.Result) (bc types.BlobChunks, err error) {
	err = res.CheckKeys("blobchunkID", "blobchunkStartByte", "blobchunkBytes", "blobchunkCreatedAt")
	if err != nil {
		return
	}
	id, _ := res.GetInt64("blobchunkID")
	blobchunkStartByte, _ := res.GetInt64("blobchunkStartByte")
	blobchunkBytes, _ := res.GetInt64("blobchunkBytes")
	blobchunkCreatedAt, _ := res.GetInt64("blobchunkCreatedAt")
	_ = blobchunkCreatedAt
	bc = types.BlobChunks{
		ID:        id,
		StartByte: blobchunkStartByte,
		Bytes:     blobchunkBytes,
	}
	return
}

func OCIBlobUploadFromResult(res types.Result) (ct types.OCIBlobUpload, err error) {
	err = res.CheckKeys("blobuploadID", "blobuploadUploadUser", "blobuploadUploadProfile", "blobuploadDigest", "blobuploadUUID", "blobuploadLocation")
	if err != nil {
		return
	}
	id, _ := res.GetInt64("blobuploadID")
	blobuploadUploadUser, _ := res.GetStr("blobuploadUploadUser")
	blobuploadUploadProfile, _ := res.GetStr("blobuploadUploadProfile")
	blobuploadDigest, _ := res.GetStr("blobuploadDigest")
	blobuploadUUID, _ := res.GetStr("blobuploadUUID")
	blobuploadLocation, _ := res.GetStr("blobuploadLocation")
	ct = types.OCIBlobUpload{
		ID:            id,
		UploadUser:    blobuploadUploadUser,
		UploadProfile: blobuploadUploadProfile,
		Digest:        blobuploadDigest,
		UUID:          uuid.MustParse(blobuploadUUID),
		Location:      blobuploadLocation,
	}
	return
}
