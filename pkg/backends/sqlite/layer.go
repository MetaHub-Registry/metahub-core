package sqlite

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

type Layer struct {
	ID        int64  `json:"id"`
	Digest    string `json:"digest"`
	MediaType string `json:"media_type"`
	Size      int    `json:"size"`
}

func (l Layer) CreateTables() (err error) {
	tableName := "layers"
	q := fmt.Sprintf(`PRAGMA table_info(%s)`, tableName)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		logrus.Infof("Table '%s' already exists!", tableName)
		return
	}
	s := `CREATE TABLE IF NOT EXISTS %s (
        id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
        "digest" TEXT,
        "media_type" TEXT,
        "size" INT);`
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

func (l Layer) GetID(digest string) (id int64, err error) {
	q := fmt.Sprintf(`SELECT id FROM layers WHERE digest="%s"`, digest)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		err = rows.Scan(&id)
		return
	} else {
		return 0, fmt.Errorf("no ID found for digest '%s'", digest)
	}
	return
}

func (l Layer) Upsert(i *types.Layer) (err error) {
	id, err := l.GetID(i.Digest)
	if err != nil {
		if err.Error() == fmt.Sprintf("no ID found for digest '%s'", i.Digest) {
			return l.Insert(i)
		} else {
			logrus.Error(err.Error())
			return
		}
	}
	i.SqlID = id
	return
}

func (l Layer) Insert(i *types.Layer) (err error) {
	q := `INSERT INTO layers (digest, media_type, size) VALUES (?, ?, ?)`
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	res, err := stmt.Exec(i.Digest, i.MediaType, i.Size)
	if err != nil {
		logrus.Error(err)
		return
	}
	i.SqlID, err = res.LastInsertId()
	return
}
