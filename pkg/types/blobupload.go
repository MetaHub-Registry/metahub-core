package types

import (
	"time"

	"github.com/google/uuid"
)

type BlobUpload struct {
	ID         int64     `json:"id"`
	UUID       uuid.UUID `json:"uuid"`
	CreatedAt  time.Time `json:"created_at"`
	FinishedAt time.Time `json:"finished_at"`
	UploadedBy string    `json:"uploaded_by"`
	BlobID     int64     `json:"blob_id"`   // blob is either a layer or a config
	BlobType   string    `json:"blob_type"` //
}

type BlobChunks struct {
	ID           int64     `json:"id"`
	BlobUploadID int64     `json:"blob_upload_id"`
	CreatedAt    time.Time `json:"created_at"`
	FinishedAt   time.Time `json:"finished_at"`
	StartByte    int64     `json:"start_byte"`
	Bytes        int64     `json:"bytes"`
}

/*

func (bu BlobUpload) WriteOCIAcceptedResponse(w http.ResponseWriter, b *OCIBlobUpload) {
	w.Header().Set("Location", b.Location)
	w.Header().Set("Content-Length", "0")
	w.Header().Set("Docker-Upload-UUID", b.UUID.String())
	w.WriteHeader(http.StatusAccepted)
}

func (bu BlobUpload) WriteOCICreateResponse(w http.ResponseWriter, b *OCIBlobUpload) {
	w.Header().Set("Location", b.Location)
	w.Header().Set("Content-Length", "0")
	w.Header().Set("Docker-Upload-UUID", b.UUID.String())
	w.WriteHeader(http.StatusCreated)
}

func (bu BlobUpload) WriteOCIPatchResponse(w http.ResponseWriter, b *OCIBlobUpload) {
	w.Header().Set("Location", b.Location)
	// TODO: Range is not correct, needs to be the byte range of the chunk
	w.Header().Set("Range", "10-10")
	w.Header().Set("Docker-Upload-UUID", b.UUID.String())
	w.Header().Set("Content-Length", "0")
	w.WriteHeader(http.StatusAccepted)
}

func (bu BlobUpload) WriteOCIPutResponse(w http.ResponseWriter, b *OCIBlobUpload) {
	w.Header().Set("Location", b.Location)
	w.Header().Set("Content-Length", "0")
	// TODO: What is the corect value for Range?
	w.Header().Set("Range", "10-10")
	w.Header().Set("Docker-Content-Digest", b.Digest)
	w.WriteHeader(http.StatusCreated)
}
*/
