package types

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type OCIBlobUpload struct {
	ID            int64     `json:"id"`
	UUID          uuid.UUID `json:"uuid"`
	RangeStart    int64     `json:"range_start"`
	RangeStop     int64     `json:"range_stop"`
	Data          []byte    `json:"-"`
	UploadUser    string    `json:"upload_user"`
	UploadProfile string    `json:"upload_profile"`
	CreatedAt     time.Time `json:"created_at"`
	FinishedAt    time.Time `json:"finished_at"`
	Repo          string    `json:"repo"`
	Location      string    `json:"location"`
	BlobID        int64     `json:"blob_id"`   // blob is either a layer or a config
	BlobType      string    `json:"blob_type"` //
	Digest        string    `json:"digest"`
	MediaType     string    `json:"media_type"`
}

func (b OCIBlobUpload) WriteOCIAcceptedResponse(w http.ResponseWriter) {
	w.Header().Set("Location", b.Location)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(b.Data)))
	w.Header().Set("Docker-Upload-UUID", b.UUID.String())
	w.WriteHeader(http.StatusAccepted)
}

func (b OCIBlobUpload) WriteOCICreateResponse(w http.ResponseWriter) {
	w.Header().Set("Location", b.Location)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(b.Data)))
	w.Header().Set("Docker-Upload-UUID", b.UUID.String())
	w.WriteHeader(http.StatusCreated)
}

func (b OCIBlobUpload) WriteOCIPatchResponse(w http.ResponseWriter) {
	w.Header().Set("Location", b.Location)
	w.Header().Set("Range", fmt.Sprintf("%d-%d", b.RangeStart, b.RangeStop))
	w.Header().Set("Docker-Upload-UUID", b.UUID.String())
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(b.Data)))
	w.WriteHeader(http.StatusAccepted)
}

func (b OCIBlobUpload) WriteOCIPutResponse(w http.ResponseWriter) {
	logrus.Warnf("Set Location in WriteOCIPutResponse to '%s'", b.Location)
	w.Header().Set("Location", b.Location)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(b.Data)))
	w.Header().Set("Range", fmt.Sprintf("%d-%d", b.RangeStart, b.RangeStop))
	w.Header().Set("Docker-Content-Digest", b.Digest)
	w.Header().Set("Docker-Upload-UUID", b.UUID.String())
	w.WriteHeader(http.StatusCreated)
}
