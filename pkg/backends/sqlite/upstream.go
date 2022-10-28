package sqlite

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

type Upstream struct {
	ID   int64  `json:"id"`
	Reg  string `json:"reg"`
	Repo string `json:"repo"`
	Tag  string `json:"tag"`
}

func (u Upstream) CreateTables() (err error) {
	tableName := "upstreams"
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
		"last_visited" TIMESTAMP,
        "reg" TEXT,
		"repo" TEXT,
		"tag" TEXT
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

func (u Upstream) Insert(i *types.Upstream) (err error) {
	q := `INSERT INTO upstreams (reg, repo, tag) VALUES (?, ?, ?)`
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Fatal(err)
	}
	res, err := stmt.Exec(i.Reg, i.Repo, i.Tag)
	if err != nil {
		logrus.Fatal(err)
	}
	i.SqlID, err = res.LastInsertId()
	return
}

func (up Upstream) GetByRequest(res *types.Upstream) (found bool, err error) {
	q := `SELECT id FROM main.upstreams 
		WHERE reg="metahub-registry.org" AND repo="%s" AND tag="%s";`
	query := fmt.Sprintf(q, res.Repo, res.Tag)
	rows, err := db.Query(query)
	if err != nil {

		logrus.Error(err)
		return false, err
	}
	for rows.Next() {
		found = true
		err = rows.Scan(&res.SqlID)
	}
	return
}

func (up Upstream) GetCreateByRequest(r types.Request) (u types.Upstream, err error) {
	tag := r.Tag
	if r.Profile != "" {
		tag = fmt.Sprintf("%s-%s", r.Tag, r.Profile)
	}
	u = types.Upstream{
		Reg:  "metahub-registry.org",
		Repo: r.Repo,
		Tag:  tag,
	}

	found, err := up.GetByRequest(&u)
	if err != nil {
		return u, err
	}
	if found {
		return u, err
	}
	// Create Upstream
	q := `INSERT INTO main.upstreams (reg, repo, tag) VALUES ("metahub-registry.org", ?, ?);`
	stmt, err := db.Prepare(q)
	if err != nil {
		err = errors.Wrap(err, "failed to db.Prepare")

		logrus.Error(err)
		return
	}
	res, err := stmt.Exec(u.Repo, u.Tag)
	if err != nil {
		err = errors.Wrap(err, "failed to stmt.Exec")
		logrus.Error(err)
		return
	}
	u.SqlID, err = res.LastInsertId()
	return
}
