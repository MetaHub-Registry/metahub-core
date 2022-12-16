package sqlite

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/chrispappas/golang-generics-set/set"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

var db *sql.DB

type Sqlite struct {
	// File is the path to the sqlite database file
	MhFile        string
	UsrFile       string
	Sql           *sql.DB
	BlobUpload    BlobUpload
	User          User
	Token         Token
	Profile       Profile
	Config        Config
	Layer         Layer
	Platform      Platform
	Filter        Filter
	Upstream      Upstream
	Manifest      Manifest
	Collection    Collection
	CollectionTag CollectionTag
	ManifestList  ManifestList
}

func New(mhFile string, tables string) (s Sqlite, err error) {
	s = Sqlite{
		MhFile:        mhFile,
		BlobUpload:    BlobUpload{},
		User:          User{},
		Token:         Token{},
		Config:        Config{},
		Profile:       Profile{},
		Filter:        Filter{},
		Platform:      Platform{},
		Upstream:      Upstream{},
		Layer:         Layer{},
		Manifest:      Manifest{},
		Collection:    Collection{},
		CollectionTag: CollectionTag{},
		ManifestList:  ManifestList{},
	}
	err = s.createDB(mhFile, tables)
	if err != nil {
		return s, err
	}
	err = s.createViews(tables)
	if err != nil {
		return s, err
	}
	return
}

func (s Sqlite) ReOpenDB() (err error) {
	db, err = sql.Open("sqlite3", s.MhFile)
	if err != nil {
		return
	}
	return
}

func (s Sqlite) Close() {
	db.Close()
}

func (s Sqlite) Setup() error {
	return nil
}

func (s Sqlite) Query(q string) (*sql.Rows, error) {
	return db.Query(q)
}

func (s Sqlite) createViews(tables string) (err error) {
	if tables == "none" {
		return
	}
	logrus.Debug(">> Create sqlite3 views")
	tbls := []string{}
	for _, ele := range strings.Split(tables, ",") {
		if ele != "" {
			tbls = append(tbls, ele)
		}
	}
	t := set.FromSlice(tbls)
	if len(tables) == 0 || t.Has("collections") {
		if err = s.Collection.CreateViews(); err != nil {
			return
		}
	}
	if len(tables) == 0 || t.Has("manifests") {
		if err = s.Manifest.CreateViews(); err != nil {
			return
		}
	}
	if len(tables) == 0 || t.Has("manifestlists") {
		if err = s.ManifestList.CreateViews(); err != nil {
			return
		}
	}
	return
}

func (s Sqlite) EmptyData(all bool) (err error) {
	skipTable := set.FromSlice([]string{"users", "profiles", "filters", "profile_filter"})
	if all {
		skipTable = set.FromSlice([]string{})
	}
	logrus.Debugf("Empty SQlite DB (except %s)", strings.Join(skipTable.Values(), "/"))
	q := `SELECT name FROM  sqlite_schema WHERE type ='table' AND name NOT LIKE 'sqlite_%';`
	rows, err := db.Query(q)
	if err != nil {
		logrus.Fatal(err)
	}
	tables := []string{}
	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			logrus.Fatal(err)
		}
		tables = append(tables, name)
	}
	rows.Close()
	for _, table := range tables {
		if skipTable.Has(table) {
			logrus.Debugf(">> SKIP table %s", table)
			continue
		}
		logrus.Debugf(">> Empty table %s", table)
		_, err = db.Exec("DELETE FROM " + table)
		if err != nil {
			logrus.Fatal(err)
		}
	}
	return
}

func (s *Sqlite) createDB(mhFile string, tables string) (err error) {
	if tables == "none" {
		return
	}
	logrus.Debugf(">> Create User databases: %s", mhFile)
	db, err = sql.Open("sqlite3", mhFile)
	checkErr(err, "while opening user-sqlite3")
	tbls := []string{}
	for _, ele := range strings.Split(tables, ",") {
		if ele != "" {
			tbls = append(tbls, ele)
		}
	}
	t := set.FromSlice(tbls)
	// Create tables
	if len(tables) == 0 || t.Has("blob-upload") {
		err = s.BlobUpload.CreateTables()
		checkErr(err, "while creating blob-upload")
	}
	if len(tables) == 0 || t.Has("users") {
		err = s.User.CreateTables()
		checkErr(err, "while creating user-tables")
	}
	if len(tables) == 0 || t.Has("tokens") {
		err = s.Token.CreateTables()
		checkErr(err, "while creating token-tables")
	}
	if len(tables) == 0 || t.Has("profiles") {
		err = s.Profile.CreateTables()
		checkErr(err, "")
	}
	if len(tables) == 0 || t.Has("filters") {
		err = s.Filter.CreateTables()
		checkErr(err, "")
	}
	if len(tables) == 0 || t.Has("profiles") {

		err = s.Platform.CreateTables()
		checkErr(err, "")
	}
	if len(tables) == 0 || t.Has("configs") {
		err = s.Config.CreateTables()
		checkErr(err, "")
	}
	if len(tables) == 0 || t.Has("layers") {
		err = s.Layer.CreateTables()
		checkErr(err, "")
	}
	if len(tables) == 0 || t.Has("upstreams") {
		err = s.Upstream.CreateTables()
		checkErr(err, "")
	}
	if len(tables) == 0 || t.Has("manifests") {
		err = s.Manifest.CreateTables()
		checkErr(err, "")
	}
	if len(tables) == 0 || t.Has("collections") {
		err = s.Collection.CreateTables()
		checkErr(err, "")
	}
	if len(tables) == 0 || t.Has("collection_tags") {
		err = s.CollectionTag.CreateTables()
		checkErr(err, "")
	}
	if len(tables) == 0 || t.Has("manifestlists") {
		err = s.ManifestList.CreateTables()
		checkErr(err, "")
	}
	return
}

func checkErr(err error, msg string) {
	if err != nil {
		if len(msg) > 0 {
			err = errors.Wrap(err, msg)
		}
		panic(err)
	}
}

func (s Sqlite) Backup(fname string) error {
	logrus.Infof("Copy SQlite DB to '%s'", fname)
	s.Close()
	utils.CopyFile(s.MhFile, fname)
	return s.ReOpenDB()
}

func (s Sqlite) BootstrapTest(bs utils.Bootstrap) (br utils.BootstrapResult, err error) {
	br = utils.NewBootstrapResult()
	for _, b := range bs.Bootstraps {
		switch b {
		case "profiles":
			err = Setup_Profiles(&s, &br, "../../../misc/yaml/profiles")
			if err != nil {
				err = errors.Wrap(err, "Setup_Profiles")
				return
			}
		case "platforms":
			err = Setup_Platforms(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Platforms")
				return
			}
		case "users":
			err = Setup_Users(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Users")
				return
			}
		case "manifest:qnib/featuretest:arm64":
			err = Setup_Manifest_QNIB_FeatureTest_arm(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Manifest_QNIB_FeatureTest_arm")
				return
			}
		case "manifest:qnib/featuretest:amd64":
			err = Setup_Manifest_QNIB_FeatureTest_amd(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Manifest_QNIB_FeatureTest_amd")
				return
			}
		case "config:qnib/featuretest:arm64":
			err = Setup_Config_FTarm(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Config_FTarm")
				return
			}
		case "config:qnib/featuretest:amd64":
			err = Setup_Config_FT_amd64(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Config_FT_amd64")
				return
			}

		case "upstreams:qnib/featuretest:arm64":
			err = Setup_Upstream_FTarm(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Upstream_FTarm")
				return
			}
		case "upstreams:qnib/featuretest:amd64":
			err = Setup_Upstream_FT_amd64(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Upstream_FT_amd64")
				return
			}
		case "layers:qnib/featuretest:arm64":
			err = Setup_Layers_FTarm(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Layers_FTarm")
				return
			}
		case "layers:qnib/featuretest:amd64":
			err = Setup_Layers_FT_amd64(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Layers_FT_amd64")
				return
			}
		case "manifestlist:qnib/featuretest:amd64":
			err = Setup_Manifestlist_FTamd64(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Manifestlist_FTamd64")
				return
			}

		case "manifestlist:qnib/featuretest:arm64":
			err = Setup_Manifestlist_FTarm(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Manifestlist_FTarm")
				return
			}
		case "collection:qnib/featuretest:arm64":
			err = Setup_Collection_QNIB_FeatureTest_arm(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Collection_QNIB_FeatureTest_arm")
				return
			}
		case "collection:qnib/featuretest:amd64":
			err = Setup_Collection_QNIB_FeatureTest_amd64(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Collection_QNIB_FeatureTest_amd64")
				return
			}
		case "collection:qnib/featuretest:generic":
			err = Setup_Collection_QNIB_FeatureTest_Generic(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Collection_QNIB_FeatureTest_Generic")
				return
			}
		case "filter:generic":
			err = Setup_Filter_Generic(&s, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Filter_Generic")
				return
			}
		default:
			err = fmt.Errorf("no Setup for '%s'", b)
			return br, err
		}
	}
	return
}
