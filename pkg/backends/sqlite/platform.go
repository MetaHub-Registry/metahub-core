package sqlite

type Platform struct {
	ID       int64  `json:"id"`
	Arch     string `json:"architecture"`
	OS       string `json:"os"`
	Variant  string `json:"variant"`
	Features string `json:"features"`
}

/*
func (p Platform) CreateTables() (err error) {
	err = p.createPlatformTable()
	if err != nil {
		logrus.Error(err)
		return
	}
	err = p.createPlatformFilterXTable()
	if err != nil {
		logrus.Error(err)
		return
	}
	return
}

func (p Platform) createPlatformTable() (err error) {
	tableName := "platforms"
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
        "os" TEXT,
        "os_version" TEXT,
        "arch" TEXT,
        "variant" TEXT,
        "features" TEXT
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

func (p Platform) createPlatformFilterXTable() (err error) {
	tableName := "platform_filter"
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
        "filter_id" INT,
		"platform_id" INT,
		FOREIGN KEY (filter_id)
			REFERENCES filter (id)
		FOREIGN KEY (platform_id)
			REFERENCES platform (id)
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

func (p Platform) Insert(i *types.Platform) (err error) {
	q := `INSERT INTO platforms (os, arch, variant, features) VALUES (?, ?, ?, ?)`
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Fatal(err)
	}
	res, err := stmt.Exec(i.OS, i.Arch, i.Variant, i.Features)
	if err != nil {
		logrus.Fatal(err)
	}
	i.ID, err = res.LastInsertId()
	return
}

func (p Platform) Upsert(i *types.Platform) (err error) {
	// Already exists?
	q := `SELECT id FROM platforms WHERE os=? AND arch=? AND variant=? AND features=?`
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Fatal(err)
	}
	var id int64
	rows, err := stmt.Query(i.OS, i.Arch, i.Variant, i.Features)
	if err != nil {
		logrus.Error(err)
		return
	}
	defer rows.Close()
	if rows.Next() {
		err = rows.Scan(&id)
		if err != nil {
			logrus.Error(err)
			return
		}
		i.ID = id
		return
	}
	// Not present, insert new Platform
	return p.Insert(i)
}

func (p Platform) InsertPlatformFilter(pfp types.ProfileFilterPlatform) (err error) {
	if pfp.Platform.ID == 0 {
		return fmt.Errorf("Platform.SqlID is 0")
	}
	if pfp.Filter.ID == 0 {
		return fmt.Errorf("Filter.SqlID is 0")
	}

	q := "INSERT INTO platform_filter (platform_id, filter_id) VALUES (?, ?)"
	stmt, err := db.Prepare(q)

	if err != nil {
		return err
	}
	_, err = stmt.Exec(pfp.Platform.ID, pfp.Filter.ID)
	if err != nil {
		return err
	}
	return
}

// UpdatePlatformFilter checks if an existing filter matches and create a link if need be
func (sq Sqlite) UpdatePlatformFilter(i types.Platform) (err error) {
	// Get all filters
	filters, err := sq.Filter.GetAllFilters()
	if err != nil {
		return
	}
	if i.Features == "" {
		// If no features are set, we link with the generic filter
		f := types.Filter{Label: "generic"}
		fID, found, err := sq.Filter.GetFilter(f)
		f.ID = fID
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("'generic' filter not found")
		}
		err = sq.Platform.InsertPlatformFilter(types.ProfileFilterPlatform{
			Platform: i,
			Filter:   f,
		})
		if err != nil {
			return err
		}
	} else {
		// Check if a features filter matches
		// TODO: Is that correct?
		for _, f := range filters {
			if f.Feature == i.Features {
				err = sq.Platform.InsertPlatformFilter(types.ProfileFilterPlatform{
					Platform: i,
					Filter:   f,
				})
				if err != nil {
					return
				}
			}
		}
	}
	return
}
*/
