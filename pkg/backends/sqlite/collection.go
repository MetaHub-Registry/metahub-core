package sqlite

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

type Collection struct {
	ID            int64               `json:"id"`
	Repo          string              `json:"repo"`
	Tag           string              `json:"tag"`
	Manifests     map[string]Manifest `json:"manifests"`
	ManifestCount int64               `json:"manifest_count"`
}

type colRes struct {
	ColID    int    `json:"collection_id"`
	ColRepo  string `json:"collection_repo"`
	ColTag   string `json:"collection_tag"`
	ManiID   int    `json:"manifest_id"`
	UpReg    string `json:"upstream_reg"`
	UpRepo   string `json:"upstream_repo"`
	UpTag    string `json:"upstream_tag"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Variant  string `json:"variant"`
	Features string `json:"features"`
	Profile  string `json:"profile"`
}

func (c *Collection) GetAll() (cols []Collection, err error) {
	q := `SELECT * FROM main.v_get_collections`
	rows, err := db.Query(q)
	if err != nil {
		logrus.Error(err)
	}
	if err != nil {
		logrus.Error(err)
	}
	col := Collection{
		ID: 0,
	}
	for rows.Next() {
		cRes := colRes{}
		err = rows.Scan(
			&cRes.ColID,
			&cRes.ColRepo,
			&cRes.ColTag,
			&cRes.ManiID,
			&cRes.UpReg,
			&cRes.UpRepo,
			&cRes.UpTag,
		)
		if err != nil {
			logrus.Error(err)
		}
		if col.ID != int64(cRes.ColID) {
			if col.ID != 0 {
				// In case it is not 0 it holds the previous collection, thus we add it
				cols = append(cols, col)
			}
			col = Collection{
				ID:            int64(cRes.ColID),
				Repo:          cRes.ColRepo,
				Tag:           cRes.ColTag,
				Manifests:     map[string]Manifest{},
				ManifestCount: 0,
			}
		}
		// Add the manifest to the collection
		m := Manifest{
			ID: int64(cRes.ManiID),
			Upstream: Upstream{
				Reg:  cRes.UpReg,
				Repo: cRes.UpRepo,
				Tag:  cRes.UpTag,
			},
		}
		key := fmt.Sprintf("%s/%s:%s", cRes.UpReg, cRes.UpRepo, cRes.UpTag)
		col.Manifests[key] = m
		col.ManifestCount++
		//fmt.Printf("%+v\n", cRes)
	}
	cols = append(cols, col)
	return
}

func (c *Collection) GetCollectionByID(colID int64) (col Collection, err error) {
	q := fmt.Sprintf(`SELECT * FROM main.v_get_collection WHERE colID = %d`, colID)
	logrus.Errorf("q: %s", q)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Error(err)
	}
	if err != nil {
		logrus.Error(err)
	}
	col = Collection{
		Manifests:     map[string]Manifest{},
		ManifestCount: 0,
	}
	for rows.Next() {
		cRes := colRes{}
		err = rows.Scan(
			&cRes.ColID,
			&cRes.ColRepo,
			&cRes.ColTag,
			&cRes.ManiID,
			&cRes.UpReg,
			&cRes.UpRepo,
			&cRes.UpTag,
			&cRes.OS,
			&cRes.Arch,
			&cRes.Variant,
			&cRes.Features,
			&cRes.Profile,
		)
		if err != nil {
			logrus.Error(err)
		}
		if col.ManifestCount == 0 {
			col.ID = int64(cRes.ColID)
			col.Repo = cRes.ColRepo
			col.Tag = cRes.ColTag
		}
		// Add the manifest to the collection
		m := Manifest{
			ID: int64(cRes.ManiID),
			Upstream: Upstream{
				Reg:  cRes.UpReg,
				Repo: cRes.UpRepo,
				Tag:  cRes.UpTag,
			},
			PlatformSpec: Platform{
				OS:       cRes.OS,
				Arch:     cRes.Arch,
				Variant:  cRes.Variant,
				Features: cRes.Features,
			},
			Profile: Profile{Name: cRes.Profile},
		}
		key := fmt.Sprintf("%s/%s:%s", cRes.UpReg, cRes.UpRepo, cRes.UpTag)
		col.Manifests[key] = m
		col.ManifestCount++
	}
	return
}

func (c *Collection) GetCollectionByRepo(repo string) (cols []Collection, err error) {
	cols = []Collection{}
	q := fmt.Sprintf(`SELECT * FROM main.v_get_collections WHERE colRepo LIKE '%s%%';`, repo)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Error(err)
	}
	if err != nil {
		logrus.Error(err)
	}
	col := Collection{
		ID: 0,
	}
	for rows.Next() {
		cRes := colRes{}
		err = rows.Scan(
			&cRes.ColID,
			&cRes.ColRepo,
			&cRes.ColTag,
			&cRes.ManiID,
			&cRes.UpReg,
			&cRes.UpRepo,
			&cRes.UpTag,
		)
		if err != nil {
			logrus.Error(err)
		}
		if col.ID != int64(cRes.ColID) {
			if col.ID != 0 {
				// In case it is not 0 it holds the previous collection, thus we add it
				cols = append(cols, col)
			}
			col = Collection{
				ID:            int64(cRes.ColID),
				Repo:          cRes.ColRepo,
				Tag:           cRes.ColTag,
				Manifests:     map[string]Manifest{},
				ManifestCount: 0,
			}
		}
		// Add the manifest to the collection
		m := Manifest{
			ID: int64(cRes.ManiID),
			Upstream: Upstream{
				Reg:  cRes.UpReg,
				Repo: cRes.UpRepo,
				Tag:  cRes.UpTag,
			},
		}
		key := fmt.Sprintf("%s/%s:%s", cRes.UpReg, cRes.UpRepo, cRes.UpTag)
		col.Manifests[key] = m
		col.ManifestCount++
		//fmt.Printf("%+v\n", cRes)
	}
	cols = append(cols, col)

	return
}

func (c *Collection) CreateViews() (err error) {
	err = c.CreateCollectionsView()
	if err != nil {
		logrus.Error(err)
		return
	}
	err = c.CreateCollectionView()
	if err != nil {
		logrus.Error(err)
		return
	}
	return
}

func (c *Collection) CreateCollectionView() (err error) {
	q := `CREATE VIEW IF NOT EXISTS v_get_collection AS
	SELECT
		collections.id AS colID,
		collections.repo AS colRepo,
		collections.tag AS colTag,
		manifests.id AS maniID,
		upstreams.reg AS upstreamReg,
		upstreams.repo AS upstreamRepo,
		upstreams.tag AS upstreamTag,
		platforms.os AS os,
		platforms.arch AS arch,
		platforms.variant AS variant,
		platforms.features AS features,
		profiles.name	 AS profile
	FROM collections
	INNER JOIN collection_xtable
		ON collections.id = collection_xtable.collection_id
	INNER JOIN manifestlists
		ON collection_xtable.manifestlist_id = manifestlists.id
	INNER JOIN manifestlist_xtable
		ON manifestlists.id = manifestlist_xtable.manifestlist_id
	INNER JOIN manifests
		ON manifestlist_xtable.manifest_id = manifests.id
	INNER JOIN upstreams
		ON manifests.upstream_id = upstreams.id
	INNER JOIN platforms
		ON manifests.platform_id = platforms.id
	INNER JOIN platform_filter
		ON platforms.id = platform_filter.platform_id
	INNER JOIN profile_filter
		ON platform_filter.filter_id = profile_filter.filter_id
	INNER JOIN profiles
		ON profiles.id = profile_filter.profile_id`
	_, err = db.Exec(q)
	return
}

func (c *Collection) CreateCollectionsView() (err error) {
	q := `CREATE VIEW IF NOT EXISTS v_get_collections AS
	SELECT
		collections.id AS colID,
		collections.repo AS colRepo,
		collections.tag AS colTag,
		manifests.id AS maniID,
		manifestlists.digest AS manifestlistDigest,
		upstreams.reg AS upstreamReg,
		upstreams.repo AS upstreamRepo,
		upstreams.tag AS upstreamTag,
		platforms.os AS os,
		platforms.arch AS arch,
		platforms.variant AS variant,
		platforms.features AS features,
		profiles.name	 AS profile
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
		INNER JOIN profiles ON profiles.id = profile_filter.profile_id`
	_, err = db.Exec(q)
	return
}

func (c Collection) createCollectionTable() (err error) {
	tableName := "collections"
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
        "repo" TEXT,
		"tag" int
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

func (c Collection) createCollectionXTable() (err error) {
	tableName := "collection_xtable"
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
        "collection_id" INT, 
		"manifestlist_id" INT,
		FOREIGN KEY (collection_id)
			REFERENCES collection (id)
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

func (c Collection) CreateTables() (err error) {
	err = c.createCollectionTable()
	if err != nil {
		return
	}
	err = c.createCollectionXTable()
	if err != nil {
		return
	}
	return
}

func (c Collection) Insert(i *types.Collection) (err error) {
	q := `INSERT INTO collections (repo, tag) VALUES (?, ?)`
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	res, err := stmt.Exec(i.Repo, i.Tag)
	if err != nil {
		logrus.Error(err)
		return
	}
	i.SqlID, err = res.LastInsertId()
	return
}

func (c Collection) InsertXTable(colID, mlID int64) (err error) {
	q := `INSERT INTO collection_xtable (collection_id, manifestlist_id) VALUES (?, ?)`
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	_, err = stmt.Exec(colID, mlID)
	if err != nil {
		logrus.Error(err)
		return
	}
	return
}

func (c *Collection) GetInsertCollectionByRepoTag(repo, tag string) (col types.Collection, err error) {
	col, err = c.GetCollectionByRepoTag(repo, tag)
	if col.SqlID == -1 {
		// Collection does not exist, thus we create it
		err = c.Insert(&col)
	}
	return
}

func (c *Collection) GetCollectionsByRepoLike(repo string) (cols []Collection, err error) {
	cols = []Collection{}
	q := fmt.Sprintf(`SELECT
		colID,
		colRepo,
		colTag,
		maniID,
		upstreamReg,
		upstreamRepo,
		upstreamTag,
		os,
		arch,
		variant,
		features,
		profile
	FROM
		main.v_get_collections WHERE colRepo LIKE '%s%%';`, repo)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Error(err)
	}
	col := Collection{
		ID: 0,
	}
	for rows.Next() {
		cRes := colRes{}
		err = rows.Scan(
			&cRes.ColID,
			&cRes.ColRepo,
			&cRes.ColTag,
			&cRes.ManiID,
			&cRes.UpReg,
			&cRes.UpRepo,
			&cRes.UpTag,
			&cRes.OS,
			&cRes.Arch,
			&cRes.Variant,
			&cRes.Features,
			&cRes.Profile,
		)
		if err != nil {
			logrus.Error(err)
		}
		if col.ID != int64(cRes.ColID) {
			if col.ID != 0 {
				// In case it is not 0 it holds the previous collection, thus we add it
				cols = append(cols, col)
			}
			col = Collection{
				ID:            int64(cRes.ColID),
				Repo:          cRes.ColRepo,
				Tag:           cRes.ColTag,
				Manifests:     map[string]Manifest{},
				ManifestCount: 0,
			}
		}
		// Add the manifest to the collection
		m := Manifest{
			ID: int64(cRes.ManiID),
			Upstream: Upstream{
				Reg:  cRes.UpReg,
				Repo: cRes.UpRepo,
				Tag:  cRes.UpTag,
			},
		}
		key := fmt.Sprintf("%s/%s:%s", cRes.UpReg, cRes.UpRepo, cRes.UpTag)
		col.Manifests[key] = m
		col.ManifestCount++
		//fmt.Printf("%+v\n", cRes)
	}
	cols = append(cols, col)

	return
}

func (c *Collection) GetCollectionByRepoTag(repo, tag string) (col types.Collection, err error) {
	q := fmt.Sprintf(`SELECT id FROM collections WHERE repo = '%s' AND tag = "%s";`, repo, tag)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Error(err)
	}
	if err != nil {
		logrus.Error(err)
	}
	col = types.Collection{
		SqlID:         -1,
		Repo:          repo,
		Tag:           tag,
		ManifestLists: []types.ManifestList{},
	}
	var id int64
	if rows.Next() {
		err = rows.Scan(&id)
		if err != nil {
			logrus.Error(err)
		}
		col.SqlID = id
	}
	return
}

func (c Collection) GetTagsListByRepo(repo string) (res types.TagResponse, err error) {
	q := fmt.Sprintf(`SELECT DISTINCT colTag FROM main.v_get_collections WHERE colRepo="%s" ORDER BY colTag;`, repo)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	res = types.TagResponse{
		Name: repo,
		Tags: []string{},
	}
	// TODO: add pagination https://docs.docker.com/registry/spec/api/#pagination-1
	for rows.Next() {
		var tag string
		err = rows.Scan(&tag)
		if err != nil {
			logrus.Error(err)
			return
		}
		res.Tags = append(res.Tags, tag)
	}
	return
}
