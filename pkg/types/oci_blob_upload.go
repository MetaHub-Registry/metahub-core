package types

import (
	"time"

	"github.com/google/uuid"
)

type OCIBlobUpload struct {
	NeoID      int64     `json:"neo_id"`
	SqlID      int64     `json:"sql_id"`
	UUID       uuid.UUID `json:"uuid"`
	RangeStart int64     `json:"range_start"`
	RangeStop  int64     `json:"range_stop"`
	Data       []byte    `json:"-"`
	UploadedBy string    `json:"uploaded_by"`
	CreatedAt  time.Time `json:"created_at"`
	FinishedAt time.Time `json:"finished_at"`
	Repo       string    `json:"repo"`
	Location   string    `json:"location"`
	BlobID     int64     `json:"blob_id"`   // blob is either a layer or a config
	BlobType   string    `json:"blob_type"` //
	Digest     string    `json:"digest"`
}
