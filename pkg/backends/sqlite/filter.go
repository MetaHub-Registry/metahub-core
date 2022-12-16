package sqlite

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

type Filter struct {
	ID    int64  `json:"id"`
	Label string `json:"label"`
}

func (f Filter) CreateTables() (err error) {
	err = f.createFilterTable()
	if err != nil {
		logrus.Error(err)
		return
	}
	err = f.createProfileFilterXTable()
	if err != nil {
		logrus.Error(err)
		return
	}
	return
}

func (f Filter) createFilterTable() (err error) {
	tableName := "filters"
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
        "label" TEXT
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

func (f Filter) GetAllFilters() (res []types.Filter, err error) {
	q := `SELECT id,label FROM filters`
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	rows, err := stmt.Query()
	if err != nil {
		logrus.Error(err)
		return
	}
	defer rows.Close()
	res = make([]types.Filter, 0)
	if rows.Next() {
		var id int
		var label string
		err = rows.Scan(&id, &label)
		if err != nil {
			logrus.Error(err)
		}
		f := types.Filter{
			ID:    int64(id),
			Label: label,
		}
		res = append(res, f)
	}
	return
}

func (f Filter) GetFilter(fltr types.Filter) (filterID int64, found bool, err error) {
	q := `SELECT id FROM filters WHERE label = ?`
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	rows, err := stmt.Query(fltr.Label)
	if err != nil {
		logrus.Error(err)
		return
	}
	defer rows.Close()
	if rows.Next() {
		err = rows.Scan(&fltr.ID)
		if err != nil {
			logrus.Error(err)
		}
		filterID = fltr.ID
		found = true
	}
	return
}

func (f Filter) createProfileFilterXTable() (err error) {
	tableName := "profile_filter"
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
        "profile_id" INT,
		"filter_id" INT,
		FOREIGN KEY (filter_id)
			REFERENCES filters (id)
		FOREIGN KEY (profile_id)
			REFERENCES profiles (id)
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

func (f Filter) InsertFilter(fltr *types.Filter) (err error) {
	// TODO: Do not insert if already exists?
	stmt, err := db.Prepare("INSERT INTO filters (label) VALUES (?) RETURNING id")
	if err != nil {
		logrus.Fatal(err)
	}
	res, err := stmt.Exec(fltr.Label)
	if err != nil {
		logrus.Fatal(err)
	}
	fltr.ID, err = res.LastInsertId()
	return

}

func (f Filter) InsertProfileFilter(pfp types.ProfileFilterPlatform) (err error) {
	stmt, err := db.Prepare("INSERT INTO profile_filter (profile_id, filter_id) VALUES (?, ?)")
	if err != nil {
		logrus.Fatal(err)
	}
	_, err = stmt.Exec(pfp.Profile.ID, pfp.Filter.ID)
	if err != nil {
		logrus.Fatal(err)
	}
	return
}
