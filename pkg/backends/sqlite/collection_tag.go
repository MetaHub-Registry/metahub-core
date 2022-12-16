package sqlite

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

type CollectionTag struct {
	ID  int64  `json:"id"`
	Tag string `json:"tag"`
}

func (c CollectionTag) Insert(i *types.CollectionTag) (err error) {
	q := `INSERT INTO collection_tags (tag) VALUES (?)`
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	res, err := stmt.Exec(i.Tag)
	if err != nil {
		logrus.Error(err)
		return
	}
	i.ID, err = res.LastInsertId()
	return
}

func (c CollectionTag) GetByTag(tag string) (i types.CollectionTag, found bool, err error) {
	i = types.CollectionTag{
		Tag: tag,
	}
	q := fmt.Sprintf(`SELECT id FROM collection_tags WHERE tag="%s"`, tag)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	defer rows.Close()
	if rows.Next() {
		found = true
		err = rows.Scan(&i.ID)
		if err != nil {
			logrus.Error(err)
		}
	}
	return
}

func (c CollectionTag) GetCreateByRequest(req types.Request) (i types.CollectionTag, err error) {
	i, found, err := c.GetByTag(req.Tag)
	if err != nil {
		return
	}
	if found {
		return
	}
	i = types.CollectionTag{
		Tag: req.Tag,
	}
	err = c.Insert(&i)
	if err != nil {
		return
	}
	return
}

func (c CollectionTag) RemoveByID(id int64) (err error) {
	q := fmt.Sprintf(`DELETE FROM collection_tags WHERE id=%d`, id)
	_, err = db.Exec(q)
	return
}

func (c CollectionTag) createCollectionTagsTable() (err error) {
	tableName := "collection_tags"
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
	logrus.Debugf("Table '%s' created successfully!", tableName)
	return
}

func (c CollectionTag) CreateTables() (err error) {
	err = c.createCollectionTagsTable()
	if err != nil {
		return
	}
	return
}
