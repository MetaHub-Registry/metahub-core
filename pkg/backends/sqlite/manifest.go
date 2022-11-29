package sqlite

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

type Manifest struct {
	ID           int64    `json:"id"`
	Digest       string   `json:"digest"`
	ImageName    string   `json:"image_name"`
	PlatformSpec Platform `json:"platform_spec"`
	Upstream     Upstream `json:"upstream"`
	Config       Config   `json:"config"`
	Profile      Profile  `json:"profile"`
}

type maniRow struct {
	colID           int64
	maniID          int64
	maniDigest      string
	maniListID      int64
	upstreamReg     string
	upstreamRepo    string
	upstreamTag     string
	os              string
	arch            string
	variant         string
	features        string
	configDigest    string
	configMediaType string
	configSize      int
	profileName     string
}

func (m *Manifest) GetByCollectionId(colId int64) (manis []Manifest, err error) {
	manis = []Manifest{}
	q := fmt.Sprintf(`SELECT 
		colID, maniID, maniDigest, upstreamReg, upstreamRepo, upstreamTag, 
		os, arch, variant, features, 
		configDigest, configMediaType, configSize, profile
	FROM main.manifests_view WHERE colID =%d;`, colId)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Error(err)
	}
	if err != nil {
		logrus.Error(err)
	}
	for rows.Next() {
		maniRes := maniRow{}
		err = rows.Scan(
			&maniRes.colID,
			&maniRes.maniID,
			&maniRes.maniDigest,
			&maniRes.upstreamReg,
			&maniRes.upstreamRepo,
			&maniRes.upstreamTag,
			&maniRes.os,
			&maniRes.arch,
			&maniRes.variant,
			&maniRes.features,
			&maniRes.configDigest,
			&maniRes.configMediaType,
			&maniRes.configSize,
			&maniRes.profileName,
		)
		if err != nil {
			logrus.Error(err)
		}
		imgName := fmt.Sprintf("%s/%s:%s", maniRes.upstreamReg, maniRes.upstreamRepo, maniRes.upstreamTag)
		mani := Manifest{
			ID:        maniRes.maniID,
			Digest:    maniRes.maniDigest,
			ImageName: imgName,
			PlatformSpec: Platform{
				OS:       maniRes.os,
				Arch:     maniRes.arch,
				Variant:  maniRes.variant,
				Features: maniRes.features,
			},
			Upstream: Upstream{
				Reg:  maniRes.upstreamReg,
				Repo: maniRes.upstreamRepo,
				Tag:  maniRes.upstreamTag,
			},
			Config: Config{
				Digest:    maniRes.configDigest,
				MediaType: maniRes.configMediaType,
				Size:      maniRes.configSize,
			},
			Profile: Profile{
				Name: maniRes.profileName,
			},
		}
		manis = append(manis, mani)
	}
	return
}

func (m Manifest) CreateViews() (err error) {
	err = m.CreateManifestView()
	return
}

func (m Manifest) CreateManifestView() (err error) {
	q := `CREATE VIEW IF NOT EXISTS manifests_view AS
	SELECT
		collections.id AS colID,
		collections.repo AS colRepo,
		collection_tags.tag AS colTag,
		collection_tags.id AS colTagID,
		manifests.id AS maniID,
		manifests.digest AS maniDigest,
		manifestlists.digest AS maniListDigest,
		manifestlists.id AS maniListID,
		upstreams.reg AS upstreamReg,
		upstreams.repo AS upstreamRepo,
		upstreams.tag AS upstreamTag,
		platforms.os AS os,
		platforms.arch AS arch,
		platforms.variant AS variant,
		platforms.features AS features,
		configs.digest AS configDigest,
		configs.id AS configID,
		configs.media_type AS configMediaType,
		configs.size AS configSize,
		profiles.name AS profile
	FROM
		collections
		INNER JOIN collection_xtable ON collections.id = collection_xtable.collection_id
		INNER JOIN collection_tags ON collection_tags.id = collection_xtable.collection_tag_id
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

func (m Manifest) CreateTables() (err error) {
	tableName := "manifests"
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
		"size" INT,
		"platform_id" int,
        "upstream_id" int,
        "config_id" INT,
		FOREIGN KEY (platform_id)
			REFERENCES platform (id)
		FOREIGN KEY (upstream_id)
			REFERENCES upstream (id) 
		FOREIGN KEY (config_id)
			REFERENCES config (id)
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

func (m Manifest) Insert(i *types.Manifest) (err error) {
	q := `INSERT INTO manifests (digest, media_type, size, platform_id, upstream_id, config_id) VALUES (?, ?, ?, ?, ?, ?)`
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	res, err := stmt.Exec(i.Digest, i.MediaType, i.Size, i.Platform.SqlID, i.Upstream.SqlID, i.Config.SqlID)
	if err != nil {
		err = errors.Wrap(err, "failed to insert manifest")
		logrus.Error(err)
		return
	}
	i.SqlID, err = res.LastInsertId()
	return
}

func (m *Manifest) GetTypesManifestByRequest(req types.Request) (res types.Manifest, found bool, err error) {
	q := `SELECT DISTINCT maniID,maniDigest,os,arch,variant,features,configDigest,configMediaType,configSize,`
	q += `upstreamReg,upstreamRepo,upstreamTag FROM main.manifests_view`
	if req.Digest != "" {
		q += fmt.Sprintf(` WHERE maniDigest="%s";`, req.Digest)
	} else if req.Tag != "" {
		q += fmt.Sprintf(` WHERE upstreamTag="%s";`, req.Tag)
	}
	rows, err := db.Query(q)
	if err != nil {
		logrus.Error(err)
	}
	for rows.Next() {
		maniRes := maniRow{}
		err = rows.Scan(
			&maniRes.maniID,
			&maniRes.maniDigest,
			&maniRes.os,
			&maniRes.arch,
			&maniRes.variant,
			&maniRes.features,
			&maniRes.configDigest,
			&maniRes.configMediaType,
			&maniRes.configSize,
			&maniRes.upstreamReg,
			&maniRes.upstreamRepo,
			&maniRes.upstreamTag,
		)
		if err != nil {
			logrus.Error(err)
			return
		}
		found = true
		res = types.Manifest{
			SqlID:     maniRes.maniID,
			Digest:    maniRes.maniDigest,
			MediaType: "application/vnd.docker.distribution.manifest.v2+json",
			Platform: types.Platform{
				OS:       maniRes.os,
				Arch:     maniRes.arch,
				Variant:  maniRes.variant,
				Features: maniRes.features,
			},
			Config: types.Config{
				Digest:    maniRes.configDigest,
				MediaType: maniRes.configMediaType,
				Size:      int64(maniRes.configSize),
			},
			Upstream: types.Upstream{
				Reg:  maniRes.upstreamReg,
				Repo: maniRes.upstreamRepo,
				Tag:  maniRes.upstreamTag,
			},
			Layers: []types.Layer{},
		}
	}
	return
}

func (m Manifest) GetIDByDigest(dig string) (id int64, found bool, err error) {
	q := `SELECT id FROM main.manifests WHERE digest=$1;`
	rows, err := db.Query(q, dig)
	if err != nil {
		logrus.Error(err)
	}
	defer rows.Close()
	if rows.Next() {
		err = rows.Scan(&id)
		found = true
	}
	if err != nil {
		logrus.Error(err)
	}
	return
}

func (m Manifest) GetInsert(i *types.Manifest) (err error) {
	id, found, err := m.GetIDByDigest(i.Digest)
	if found {
		i.SqlID = id
		return
	}
	return m.Insert(i)
}

func (m *Manifest) TypesManifest_GetByManifestListID(mlID int64) (manis []types.Manifest, err error) {
	manis = []types.Manifest{}
	q := `SELECT maniID,maniDigest,maniListID,os,arch,variant,features,configDigest,configMediaType,configSize,`
	q += `upstreamReg,upstreamRepo,upstreamTag FROM main.manifests_view`
	q += fmt.Sprintf(` WHERE maniListID=%d;`, mlID)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Error(err)
	}
	defer rows.Close()
	for rows.Next() {
		maniRes := maniRow{}
		err = rows.Scan(
			&maniRes.maniID,
			&maniRes.maniDigest,
			&maniRes.maniListID,
			&maniRes.os,
			&maniRes.arch,
			&maniRes.variant,
			&maniRes.features,
			&maniRes.configDigest,
			&maniRes.configMediaType,
			&maniRes.configSize,
			&maniRes.upstreamReg,
			&maniRes.upstreamRepo,
			&maniRes.upstreamTag,
		)
		if err != nil {
			logrus.Error(err)
		}
		mani := types.Manifest{
			SqlID:     maniRes.maniID,
			Digest:    maniRes.maniDigest,
			MediaType: "application/vnd.docker.distribution.manifest.v2+json",
			Platform: types.Platform{
				OS:       maniRes.os,
				Arch:     maniRes.arch,
				Variant:  maniRes.variant,
				Features: maniRes.features,
			},
			Config: types.Config{
				Digest:    maniRes.configDigest,
				MediaType: maniRes.configMediaType,
				Size:      int64(maniRes.configSize),
			},
			Upstream: types.Upstream{
				Reg:  maniRes.upstreamReg,
				Repo: maniRes.upstreamRepo,
				Tag:  maniRes.upstreamTag,
			},
			Layers: []types.Layer{},
		}
		manis = append(manis, mani)
	}
	return
}

func (m Manifest) DeleteXTableByManifestID(maniID int64) (mlIDs []int64, err error) {
	// Remove
	q := fmt.Sprintf(`DELETE FROM main.manifestlist_xtable WHERE manifest_id=%d RETURNING manifestlist_id`, maniID)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var mlID int64
		err = rows.Scan(&mlID)
		if err != nil {
			logrus.Error(err)
			return
		}
		mlIDs = append(mlIDs, mlID)
	}
	return
}
func (m Manifest) RemoveByID(id int64) (mlIDs []int64, err error) {
	q := fmt.Sprintf(`DELETE FROM main.manifests WHERE id=%d RETURNING upstream_id, config_id`, id)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	defer rows.Close()
	var upstreamID, configID int64
	found := false
	if rows.Next() {
		found = true
		err = rows.Scan(&upstreamID, &configID)
		if err != nil {
			logrus.Error(err)
			return
		}
	}
	rows.Close()
	if !found {
		err = fmt.Errorf("Manifest with id %d not found", id)
		return
	}
	mlIDs, err = m.DeleteXTableByManifestID(id)
	// ScrubUpstream
	// TODO: Move me into Magazin
	q = fmt.Sprintf(`SELECT COUNT(id) FROM main.manifests WHERE upstream_id=%d`, upstreamID)
	rows, err = db.Query(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	defer rows.Close()
	var count int64
	if rows.Next() {
		err = rows.Scan(&count)
		if err != nil {
			logrus.Error(err)
			return
		}
	}
	rows.Close()
	if count == 0 {
		q = fmt.Sprintf(`DELETE FROM main.upstreams WHERE id=%d`, upstreamID)
		_, err = db.Exec(q)
		if err != nil {
			logrus.Error(err)
			return
		}
	}
	// ScrubConfig
	// TODO: Move me into Magazin
	q = fmt.Sprintf(`SELECT COUNT(id) FROM main.manifests WHERE config_id=%d`, configID)
	rows, err = db.Query(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	defer rows.Close()
	var cfgCount int64

	if rows.Next() {
		err = rows.Scan(&cfgCount)
		if err != nil {
			logrus.Error(err)
			return
		}
	}
	rows.Close()
	return
}

func (m Manifest) Remove(i types.Manifest) (mlIDs []int64, maniDigest string, err error) {
	id, found, err := m.GetIDByDigest(i.Digest)
	if err != nil {
		return
	}
	if found {
		maniDigest = i.Digest
		mlIDs, err = m.RemoveByID(id)
		return
	}
	// By Tag
	//id, found, err = m.GetIDByTag(i.Upstream.Tag)
	q := `SELECT DISTINCT maniID,maniDigest FROM main.manifests_view`
	q += fmt.Sprintf(` WHERE upstreamRepo="%s" AND upstreamTag="%s";`, i.Upstream.Repo, i.Upstream.Tag)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	defer rows.Close()
	var maniID int64
	found = false
	if rows.Next() {
		found = true
		err = rows.Scan(&maniID, &maniDigest)
		if err != nil {
			logrus.Error(err)
			return
		}
	}
	rows.Close()
	if found {
		mlIDs, err = m.RemoveByID(maniID)
		return
	}
	err = fmt.Errorf("manifest not found")
	return
}
