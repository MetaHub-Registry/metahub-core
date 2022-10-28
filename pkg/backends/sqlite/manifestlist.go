package sqlite

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

// ManifestList serve a collection for a certain profile.
type ManifestList struct {
	ID     int64  `json:"id"`
	Digest string `json:"digest"`
	// key of the map matches the key of the map in Platforms to construct the platformObject indexes of the ManifestList
	Manifests map[string]Manifest `json:"manifests"`
	Platforms map[string]Platform `json:"platform"`
}

type mlRes struct {
	ColID    int    `json:"collection_id"`
	MlID     int    `json:"manifest_list_id"`
	ColRepo  string `json:"collection_repo"`
	ColTag   string `json:"collection_tag"`
	MLDigest string `json:"manifest_list_digest"`
	MlDigest string `json:"manifest_digest"`
	UpReg    string `json:"upstream_reg"`
	UpRepo   string `json:"upstream_repo"`
	UpTag    string `json:"upstream_tag"`
	Profile  string `json:"profile"`
}

func (ml ManifestList) CreateTables() (err error) {
	err = ml.createManifestlistTable()
	if err != nil {
		return
	}
	err = ml.createManifestListXTable()
	if err != nil {
		return
	}
	return
}

func (ml ManifestList) CreateViews() (err error) {
	err = ml.CreateManifestListView()
	if err != nil {
		logrus.Error(err)
		return
	}
	return
}

func (ml ManifestList) CreateManifestListView() (err error) {
	q := `CREATE VIEW IF NOT EXISTS v_get_manifestlist AS
	SELECT DISTINCT
		collections.id AS colID,
		collections.repo AS colRepo,
		collections.tag AS colTag,
		profiles.id AS profileID,
		profiles.name AS profileName,
		manifestlists.digest AS manifestlistDigest,
		manifestlists.id AS manifestlistID
	FROM
		collections
	INNER JOIN collection_xtable ON collections.id = collection_xtable.collection_id
	INNER JOIN manifestlists ON collection_xtable.manifestlist_id = manifestlists.id
	INNER JOIN manifestlist_xtable ON manifestlists.id = manifestlist_xtable.manifestlist_id
	INNER JOIN manifests ON manifestlist_xtable.manifest_id = manifests.id
	INNER JOIN upstreams ON manifests.upstream_id = upstreams.id
	INNER JOIN platforms ON manifests.platform_id = platforms.id
	INNER JOIN platform_filter ON platforms.id = platform_filter.platform_id
	INNER JOIN profile_filter ON platform_filter.filter_id = profile_filter.filter_id
	INNER JOIN profiles ON profiles.id = profile_filter.profile_id
	INNER JOIN configs ON configs.id = manifests.config_id`
	_, err = db.Exec(q)
	return
}

func (ml ManifestList) createManifestListXTable() (err error) {
	tableName := "manifestlist_xtable"
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
        "manifest_id" INT, 
		"manifestlist_id" INT,
		FOREIGN KEY (manifest_id)
			REFERENCES manifest (id)
		FOREIGN KEY (manifestlist_id)
			REFERENCES manifestlist (id)
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
	logrus.Infof("Table '%s' created successfully!", tableName)
	return
}

func (ml ManifestList) createManifestlistTable() (err error) {
	tableName := "manifestlists"
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
		"media_type" TEXT,
		"size" INT
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
	logrus.Infof("Table '%s' created successfully!", tableName)
	return
}

func (ml ManifestList) Insert(i *types.ManifestList) (err error) {
	q := `INSERT INTO manifestlists (digest, media_type) VALUES (?, ?)`
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	res, err := stmt.Exec(i.Digest, i.MediaType)
	if err != nil {
		logrus.Error(err)
		return
	}
	i.SqlID, err = res.LastInsertId()
	for _, m := range i.Manifests {
		// Get SqlID of manifest
		q := `SELECT id FROM manifests WHERE digest=?`
		stmt, err = db.Prepare(q)
		if err != nil {
			logrus.Error(err)
			return err
		}
		var maniSqlID int64
		err = stmt.QueryRow(m.Digest).Scan(&maniSqlID)
		if err != nil {
			logrus.Error(err)
			return err
		}

		q = `INSERT INTO manifestlist_xtable (manifest_id, manifestlist_id) VALUES (?, ?)`
		stmt, err = db.Prepare(q)
		if err != nil {
			logrus.Error(err)
			return
		}
		_, err = stmt.Exec(maniSqlID, i.SqlID)
		if err != nil {
			logrus.Error(err)
			return
		}
	}
	return
}

func (ml ManifestList) GetTypesManifestListByRequest(req types.Request) (res types.ManifestList, found bool, err error) {
	q := fmt.Sprintf(`SELECT DISTINCT colID,colRepo,colTag,manifestlistDigest,manifestlistID FROM v_get_manifestlist WHERE colRepo="%s" AND colTag="%s" AND profileName="%s"`, req.Repo, req.Tag, req.Profile)
	if req.Tag == "" {
		q = fmt.Sprintf(`SELECT DISTINCT colID,colRepo,colTag,manifestlistDigest,manifestlistID FROM v_get_manifestlist WHERE colRepo="%s" AND manifestlistDigest="%s" AND profileName="%s"`, req.Repo, req.Digest, req.Profile)
	}
	rows, err := db.Query(q)
	if err != nil {
		logrus.Error(err)
	}
	// BootStrap the ML
	res = types.ManifestList{
		Digest:        "",
		SchemaVersion: 2,
		MediaType:     "application/vnd.docker.distribution.manifest.list.v2+json",
		Manifests:     []types.Manifest{},
	}
	for rows.Next() {
		found = true
		r := mlRes{}
		err = rows.Scan(
			&r.ColID,
			&r.ColRepo,
			&r.ColTag,
			&r.MlDigest,
			&r.MlID,
		)
		if err != nil {
			logrus.Error(err)
			continue
		}
		res.SqlID = int64(r.MlID)
		if res.Digest == "" {
			// First row
			res.Digest = r.MlDigest
		}
	}
	return
}
