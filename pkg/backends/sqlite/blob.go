package sqlite

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
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

func (bu BlobUpload) CreateTables() (err error) {
	tableName := "blob_uploads"
	q := fmt.Sprintf(`PRAGMA table_info(%s)`, tableName)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		//logrus.Infof("Table '%s' already exists!", tableName)
		return
	}
	s := `CREATE TABLE IF NOT EXISTS %s (
        id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
        "uuid" TEXT,
		"digest" TEXT,
		"uploaded_by" TEXT,
		"created_at" TIMESTAMP,
		"finished_at" TIMESTAMP,
        "blob_id" INTEGER,
		"location" TEXT,
		"blob_type" TEXT);`
	stmt := fmt.Sprintf(s, tableName)
	query, err := db.Prepare(stmt)
	if err != nil {
		logrus.Fatal(err)
	}
	_, err = query.Exec()
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Infof("Table '%s' created successfully!", tableName)
	return
}

func (bu BlobUpload) Create(i *types.OCIBlobUpload) (err error) {
	q := `INSERT INTO blob_uploads (uploaded_by, created_at, blob_id,blob_type, digest, uuid, location) VALUES (?, ?, ?, ?, ?, ?, ?)`
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	i.UUID = uuid.New()
	i.Location = fmt.Sprintf("/v2/%s/blobs/uploads/%s", i.Repo, i.UUID)
	res, err := stmt.Exec(i.UploadedBy, time.Now(), i.BlobID, i.BlobType, i.Digest, i.UUID.String(), i.Location)
	if err != nil {
		logrus.Error(err)
		return
	}
	i.SqlID, err = res.LastInsertId()
	return
}

func (bu BlobUpload) UpdateDigestByUUID(i *types.OCIBlobUpload) (err error) {
	// Fetch layer ID from layers table
	if i.Digest == "" {
		return fmt.Errorf("digest is empty")
	}
	q := fmt.Sprintf(`SELECT id FROM layers WHERE digest = "%s"`, i.Digest)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		err = rows.Scan(&i.BlobID)
		if err != nil {
			logrus.Error(err)
			return
		}
	} else {
		err = fmt.Errorf("no layer with digest '%s' found", i.Digest)
		return
	}
	rows.Close()
	// Update blob_uploads table
	q = `UPDATE blob_uploads SET digest = ?, blob_id = ? WHERE uuid = ?`
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	defer stmt.Close()
	if i.UUID.String() == "" {
		return fmt.Errorf("UUID is empty")
	}
	_, err = stmt.Exec(i.Digest, i.BlobID, i.UUID)
	if err != nil {
		logrus.Error(err)
		return
	}
	return
}

func (bu BlobUpload) CompleteByUUID(i *types.OCIBlobUpload) (err error) {
	q := `UPDATE blob_uploads SET finished_at = ? WHERE uuid = ?`
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	defer stmt.Close()
	if i.UUID.String() == "" {
		return fmt.Errorf("UUID is empty")
	}
	_, err = stmt.Exec(time.Now(), i.UUID)
	if err != nil {
		logrus.Error(err)
		return
	}
	return
}

func (bu BlobUpload) GetByUUID(uid string) (b types.OCIBlobUpload, err error) {
	q := fmt.Sprintf(`SELECT id, uploaded_by, created_at, blob_id, blob_type, location FROM blob_uploads WHERE uuid="%s"`, uid)
	logrus.Warnf("Query: %s", q)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Fatal(err)
	}
	defer rows.Close()
	b = types.OCIBlobUpload{
		UUID: uuid.MustParse(uid),
	}
	if rows.Next() {
		err = rows.Scan(
			&b.SqlID,
			&b.UploadedBy,
			&b.CreatedAt,
			&b.BlobID,
			&b.BlobType,
			&b.Location,
		)
	}
	logrus.Warnf("OCIBlobUpload: %v", b)
	return
}

func (bu BlobUpload) WriteOCIAcceptedResponse(w http.ResponseWriter, b *types.OCIBlobUpload) {
	w.Header().Set("Location", b.Location)
	w.Header().Set("Content-Length", "0")
	w.Header().Set("Docker-Upload-UUID", b.UUID.String())
	w.WriteHeader(http.StatusAccepted)
}

func (bu BlobUpload) WriteOCICreateResponse(w http.ResponseWriter, b *types.OCIBlobUpload) {
	w.Header().Set("Location", b.Location)
	w.Header().Set("Content-Length", "0")
	w.Header().Set("Docker-Upload-UUID", b.UUID.String())
	w.WriteHeader(http.StatusCreated)
}

func (bu BlobUpload) WriteOCIPatchResponse(w http.ResponseWriter, b *types.OCIBlobUpload) {
	logrus.Infof("After PATH upload, location: %s", b.Location)
	w.Header().Set("Location", b.Location)
	w.WriteHeader(http.StatusAccepted)
}

func (bu BlobUpload) WriteOCIPutResponse(w http.ResponseWriter, b *types.OCIBlobUpload) {
	w.Header().Set("Location", b.Location)
	w.Header().Set("Content-Length", "0")
	// TODO: What is the corect value for Range?
	w.Header().Set("Range", "10-10")
	w.Header().Set("Docker-Content-Digest", b.Digest)
	w.WriteHeader(http.StatusCreated)
}
