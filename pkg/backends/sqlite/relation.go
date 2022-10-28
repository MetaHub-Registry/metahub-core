package sqlite

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (s Sqlite) InsertRelation(req types.Request, plat *types.Platform) (err error) {
	q := `SELECT
		profiles.id AS profID,
		profiles.name AS profName,
		platforms.id AS platID,
		platforms.arch AS platArch,
		platforms.os AS platOS,
		platforms.variant AS platVariant,
		platforms.features AS platFeatures
	FROM
		profiles
		INNER JOIN profile_filter ON profiles.id = profile_filter.profile_id
		INNER JOIN filters ON filters.id = profile_filter.filter_id
		INNER JOIN platform_filter ON platform_filter.filter_id = filters.id
		INNER JOIN platforms ON platforms.id = platform_filter.platform_id
	WHERE profiles.name = "%s" AND platforms.arch = "%s" AND platforms.os = "%s";`
	query := fmt.Sprintf(q, req.Profile, plat.Arch, plat.OS)
	logrus.Warnf("Query: %s", query)
	rows, err := db.Query(query)
	if err != nil {
		logrus.Error(err)
		return
	}
	defer rows.Close()
	type Res struct {
		profID       int64
		profName     string
		platID       int64
		platArch     string
		platOS       string
		platVariant  string
		platFeatures string
	}
	res := Res{}
	if rows.Next() {
		err = rows.Scan(
			&res.profID,
			&res.profName,
			&res.platID,
			&res.platArch,
			&res.platOS,
			&res.platVariant,
			&res.platFeatures,
		)
		if err != nil {
			logrus.Error(err)
		}
		plat.SqlID = res.platID
		plat.Features = res.platFeatures
	}
	return
}
