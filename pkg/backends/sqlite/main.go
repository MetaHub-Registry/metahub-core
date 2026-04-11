package sqlite

import (
	"database/sql"
	"strings"

	"github.com/chrispappas/golang-generics-set/set"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"go.opentelemetry.io/otel"
)

var (
	tracer = otel.Tracer("sqlite")
	db     *sql.DB
)

type Sqlite struct {
	// MhFile is the path to the sqlite database file
	MhFile string
	Sql    *sql.DB
	User   User
	Secret Secret
}

func New(mhFile string, tables string) (s Sqlite, err error) {
	s = Sqlite{
		MhFile: mhFile,
		User:   User{},
		Secret: Secret{},
	}
	err = s.createDB(mhFile, tables)
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
	if len(tables) == 0 || t.Has("users") {
		err = s.User.CreateTables()
		checkErr(err, "while creating user-tables")
	}
	if len(tables) == 0 || t.Has("secrets") {
		err = s.Secret.CreateTables()
		checkErr(err, "while creating secret-tables")
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
