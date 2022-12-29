package neo

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (n Neo) BlobUpload_Create_ByOCI(i *types.OCIBlobUpload) (err error) {
	i.UUID = uuid.New()
	i.Location = fmt.Sprintf("/v2/%s/blobs/uploads/%s", i.Repo, i.UUID.String())
	q := `CREATE (bu:BlobUpload {uploaded_by: "%s", digest: "%s", uuid: "%s", location:"%s", created_at: TIMESTAMP()})`
	q += ` RETURN ID(bu) AS blobuploadID`
	query := fmt.Sprintf(q, i.UploadedBy, i.Digest, i.UUID.String(), i.Location)
	res, err := n.Execute(query)
	if err != nil {
		return
	}
	i.ID, err = res.GetInt64("blobuploadID")
	return
}

func (n Neo) BlobUpload_GetOCI_ByUUID(uuid string) (bu types.OCIBlobUpload, found bool, err error) {
	q := `MATCH (bu:BlobUpload {uuid: "%s"})`
	q += ` RETURN ID(bu) AS blobuploadID, bu.uploaded_by AS blobuploadUploadedBy, bu.digest AS blobuploadDigest,`
	q += ` bu.uuid AS blobuploadUUID, bu.location AS blobuploadLocation`
	query := fmt.Sprintf(q, uuid)
	res, found, err := n.QueryOne(query)
	if err != nil || !found {
		return
	}
	bu, err = OCIBlobUploadFromResult(res)
	return
}

func (n Neo) BlobUpload_Update_ByUUID(i *types.OCIBlobUpload) (err error) {
	// Is digest a layer or a config?
	q := `MATCH (l:Layer) WHERE l.digest = "%s" RETURN ID(l) AS layerID`
	query := fmt.Sprintf(q, i.Digest)
	res, found, err := n.QueryOne(query)
	if err != nil {
		return
	}
	if found {
		layerID, err := res.GetInt64("layerID")
		if err != nil {
			return err
		}
		q = `MATCH (bu:BlobUpload) WHERE id(bu) = %d`
		q += ` MATCH (l:Layer) WHERE id(l) = %d`
		q += ` MERGE (bu)-[:IS]->(l)`
		query = fmt.Sprintf(q, i.ID, layerID)
		_, err = n.Execute(query)
		if err != nil {
			return err
		}

	} else {
		q = `MATCH (c:Config) WHERE c.digest = "%s" RETURN ID(c) AS configID`
		query := fmt.Sprintf(q, i.Digest)
		res, found, err = n.QueryOne(query)
		if err != nil {
			return
		}
		if found {
			configID, err := res.GetInt64("configID")
			if err != nil {
				return err
			}
			q = `MATCH (bu:BlobUpload) WHERE id(bu) = %d`
			q += ` MATCH (c:Config) WHERE id(c) = %d`
			q += ` MERGE (bu)-[:IS]->(c)`
			query = fmt.Sprintf(q, i.ID, configID)
			_, err = n.Execute(query)
			if err != nil {
				return err
			}
		}
	}
	return
}

func (n Neo) BlobUpload_Complete(i *types.OCIBlobUpload) (err error) {
	q := `MATCH (bu:BlobUpload) WHERE id(bu) = %d`
	q += ` SET bu.finished_at = TIMESTAMP()`
	query := fmt.Sprintf(q, i.ID)
	_, err = n.Execute(query)
	return
}

// // CHUNKS
func (n Neo) UploadChunk_Create(buID, startByte, lenBytes int64) (bc types.BlobChunks, err error) {
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
	res, err := n.Execute(query)
	if err != nil {
		return
	}
	bc.ID, err = res.GetInt64("blobchunkID")
	return
}

func (n Neo) UploadChunk_Get_ByUUID(buID int64) (bcs []types.BlobChunks, err error) {
	q := `MATCH (bu:BlobUpload) WHERE id(bu) = %d`
	q += ` MATCH (bu)-[:CHUNK]->(bc:BlobChunk)`
	//fmt.Printf("QUERY: \n%s\n", fmt.Sprintf(q, buID))
	q += ` RETURN ID(bc) AS blobchunkID, bc.start_byte AS blobchunkStartByte, bc.bytes AS blobchunkBytes, bc.range AS blobchunkRange, bc.created_at AS blobchunkCreatedAt`
	query := fmt.Sprintf(q, buID)
	res, err := n.Query(query)
	if err != nil {
		return
	}
	for _, r := range res {
		bc, err := BlobChunkFromResult(r)
		if err != nil {
			return bcs, err
		}
		bcs = append(bcs, bc)
	}
	return

}

// / Helpers
func BlobChunkFromResult(res Result) (bc types.BlobChunks, err error) {
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

func OCIBlobUploadFromResult(res Result) (ct types.OCIBlobUpload, err error) {
	err = res.CheckKeys("blobuploadID", "blobuploadUploadedBy", "blobuploadDigest", "blobuploadUUID", "blobuploadLocation")
	if err != nil {
		return
	}
	id, _ := res.GetInt64("blobuploadID")
	blobuploadUploadedBy, _ := res.GetStr("blobuploadUploadedBy")
	blobuploadDigest, _ := res.GetStr("blobuploadDigest")
	blobuploadUUID, _ := res.GetStr("blobuploadUUID")
	blobuploadLocation, _ := res.GetStr("blobuploadLocation")
	ct = types.OCIBlobUpload{
		ID:         id,
		UploadedBy: blobuploadUploadedBy,
		Digest:     blobuploadDigest,
		UUID:       uuid.MustParse(blobuploadUUID),
		Location:   blobuploadLocation,
	}
	return
}
