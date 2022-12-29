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

type BlobChunks struct {
	ID           int64     `json:"id"`
	BlobUploadID int64     `json:"blob_upload_id"`
	CreatedAt    time.Time `json:"created_at"`
	FinishedAt   time.Time `json:"finished_at"`
	StartByte    int64     `json:"start_byte"`
	Bytes        int64     `json:"bytes"`
}

func (bu BlobUpload) CreateTables() (err error) {
	err = bu.CreateBUTables()
	if err != nil {
		logrus.Error(err)
		return
	}
	err = bu.CreateChunkTables()
	if err != nil {
		logrus.Error(err)
		return
	}
	return
}

func (bu BlobUpload) CreateBUTables() (err error) {
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
	logrus.Debugf("Table '%s' created successfully!", tableName)
	return
}

func (bu BlobUpload) CreateChunkTables() (err error) {
	tableName := "blob_upload_chunks"
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
		blob_upload_id INTEGER,
		created_at TIMESTAMP,
		finished_at TIMESTAMP,
		start_byte INTEGER,
		bytes INTEGER,
        FOREIGN KEY (blob_upload_id)
			REFERENCES blob_uploads (id)
		);`
	stmt := fmt.Sprintf(s, tableName)
	query, err := db.Prepare(stmt)
	if err != nil {
		logrus.Fatal(err)
	}
	_, err = query.Exec()
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Debugf("Table '%s' created successfully!", tableName)
	return
}

func (bu BlobUpload) Create(i *types.OCIBlobUpload) (err error) {
	q := `INSERT INTO blob_uploads (uploaded_by, created_at, blob_id, blob_type, digest, uuid, location) VALUES (?, ?, ?, ?, ?, ?, ?)`
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
	i.ID, err = res.LastInsertId()
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
		i.BlobType = "layer"
	} else {
		rows.Close()
		q := fmt.Sprintf(`SELECT id FROM configs WHERE digest = "%s"`, i.Digest)
		rows, err = db.Query(q)
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
			return fmt.Errorf("could not find blob with digest '%s' in layers or configs", i.Digest)
		}
		i.BlobType = "config"
	}

	rows.Close()
	// Update blob_uploads table
	q = `UPDATE blob_uploads SET digest = ?, blob_id = ?, blob_type = ? WHERE uuid = ?`
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	defer stmt.Close()
	if i.UUID.String() == "" {
		return fmt.Errorf("UUID is empty")
	}
	_, err = stmt.Exec(i.Digest, i.BlobID, i.BlobType, i.UUID)
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

func (bu BlobUpload) GetChunkUploadsByUUID(buID int64) (bcs []BlobChunks, err error) {
	q := fmt.Sprintf(`SELECT id, start_byte, bytes FROM blob_upload_chunks WHERE blob_upload_id=%d ORDER BY start_byte`, buID)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	defer rows.Close()
	bcs = []BlobChunks{}
	for rows.Next() {
		var bc BlobChunks
		err = rows.Scan(&bc.ID, &bc.StartByte, &bc.Bytes)
		if err != nil {
			logrus.Error(err)
			return
		}
		bcs = append(bcs, bc)
	}
	return
}

func (bu BlobUpload) CreateUploadChunk(buID, startByte, lenBytes int64) (bc BlobChunks, err error) {
	bc = BlobChunks{
		BlobUploadID: buID,
		CreatedAt:    time.Now(),
		StartByte:    startByte,
		Bytes:        lenBytes,
	}
	// TODO: Add create_at
	q := fmt.Sprintf(`INSERT INTO blob_upload_chunks (blob_upload_id,start_byte, bytes) VALUES (%d,%d,%d)`, buID, startByte, lenBytes)
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	defer stmt.Close()
	res, err := stmt.Exec()
	if err != nil {
		logrus.Error(err)
		return
	}
	bc.ID, err = res.LastInsertId()
	return
}

func (bu BlobUpload) GetByUUID(uid string) (b types.OCIBlobUpload, err error) {
	q := fmt.Sprintf(`SELECT id, uploaded_by, digest, blob_id, blob_type, location FROM blob_uploads WHERE uuid="%s"`, uid)
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
			&b.ID,
			&b.UploadedBy,
			&b.Digest,
			&b.BlobID,
			&b.BlobType,
			&b.Location,
		)
	}
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
	w.Header().Set("Location", b.Location)
	w.Header().Set("Range", "10-10")
	w.Header().Set("Docker-Upload-UUID", b.UUID.String())
	w.Header().Set("Content-Length", "0")
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
