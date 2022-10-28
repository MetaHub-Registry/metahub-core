package sqlite

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

type Config struct {
	ID        int64  `json:"id"`
	Digest    string `json:"digest"`
	Label     string `json:"label"`
	MediaType string `json:"media_type"`
	Size      int    `json:"size"`
}

func (c Config) CreateTables() (err error) {
	tableName := "configs"
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
        "digest" TEXT,
        "label" TEXT,
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

func (c Config) GetID(digest string) (id int64, err error) {
	q := fmt.Sprintf(`SELECT id FROM configs WHERE digest="%s"`, digest)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		err = rows.Scan(&id)
		return
	}
	return 0, fmt.Errorf("no ID found for digest '%s'", digest)
}

func (c Config) Upsert(i *types.Config) (err error) {
	id, err := c.GetID(i.Digest)
	if err != nil {
		if err.Error() == fmt.Sprintf("no ID found for digest '%s'", i.Digest) {
			return c.Insert(i)
		} else {
			logrus.Error(err.Error())
			return
		}
	}
	i.SqlID = id
	return
}

func (c Config) Insert(i *types.Config) (err error) {
	q := `INSERT INTO configs (digest, label, media_type, size) VALUES (?, ?, ?, ?)`
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	res, err := stmt.Exec(i.Digest, i.Label, i.MediaType, i.Size)
	if err != nil {
		logrus.Error(err)
		return
	}
	i.SqlID, err = res.LastInsertId()
	return
}

func (c Config) GetByRequest(cfg *types.Config) (found bool, err error) {
	q := `SELECT id FROM main.configs 
		WHERE digest="%s";`
	query := fmt.Sprintf(q, cfg.Digest)
	rows, err := db.Query(query)
	if err != nil {
		logrus.Error(err)
	}
	for rows.Next() {
		found = true
		err = rows.Scan(&cfg.SqlID)
	}
	return
}

func (c Config) GetCreateByRequest(ociMani types.OCIManifest) (cfg types.Config, err error) {
	cfg = types.Config{
		Digest:    ociMani.Config.Digest.String(),
		Size:      ociMani.Config.Size,
		MediaType: ociMani.Config.MediaType,
	}

	found, err := c.GetByRequest(&cfg)
	if err != nil {
		return cfg, err
	}
	if found {
		return cfg, err
	}
	// Create Upstream
	q := `INSERT INTO main.configs (digest, media_type, size) VALUES (?, ?, ?);`
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	res, err := stmt.Exec(cfg.Digest, cfg.MediaType, cfg.Size)
	if err != nil {
		logrus.Error(err)
		return
	}
	cfg.SqlID, err = res.LastInsertId()
	return
}
