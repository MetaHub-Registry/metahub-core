package neo

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgNamePlatform = "backends/neo/platform.go"
)

func (n Neo) PlatformGetInsert(ctx context.Context, p *types.Platform) (err error) {
	ctx, span := tracer.Start(ctx, "PlatformGetInsert")
	defer span.End()
	platID, success, err := n.GetPlatform(ctx, *p)
	if err != nil {
		return
	}
	if !success {
		platID, err = n.CreatePlatform(ctx, *p)
		if err != nil {
			return
		}
	}
	p.ID = platID
	return
}

func (n Neo) PlatformGetByID(platID int64) (plat types.Platform, found bool, err error) {
	q := `MATCH (n:Platform) WHERE id(n) = %d`
	q += ` RETURN id(n) AS platformID, n.arch AS platformArch, n.os AS platformOS, n.variant AS platformVariant`
	query := fmt.Sprintf(q, platID)
	//fmt.Printf("Query: \n%s\n", query)
	res, found, err := n.QueryOne(query)
	if err != nil || !found {
		return
	}
	plat, err = PlatformFromResult(res)
	return
}

func (n Neo) PlatformGetByManifestID(ctx context.Context, maniDataID int64) (plat types.Platform, found bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":    pkgNamePlatform,
		"func":       "PlatformGetByManifestID",
		"maniDataID": maniDataID,
	})
	defer span.End()

	q := fmt.Sprintf(`MATCH (n:Platform)<-[:HAS_PLATFORM]-(md:Manifest) WHERE id(md) = %d`, maniDataID)
	q += ` RETURN id(n) AS platformID, n.arch AS platformArch, n.os AS platformOS, n.variant AS platformVariant`
	utils.Debug(l, span, q)
	res, found, err := n.QueryOne(q)
	if err != nil || !found {
		return
	}
	plat, err = PlatformFromResult(res)
	return
}

func (n Neo) GetPlatform(ctx context.Context, plat types.Platform) (platID int64, success bool, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":  pkgNamePlatform,
		"func":     "GetPlatform",
		"platform": fmt.Sprintf("%s/%s/%s", plat.OS, plat.Arch, plat.Variant),
	})
	defer span.End()
	q := `MATCH (n:Platform) WHERE n.arch = "%s" AND n.os = "%s" AND n.variant = "%s" RETURN id(n) AS ID`
	query := fmt.Sprintf(q, plat.Arch, plat.OS, plat.Variant)
	utils.Debug(l, span, query)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		utils.ErrorHandler(l, span, err, errFailedRunQuery)
		return
	}
	if result.Next() {
		v, ok := result.Record().Get("ID")
		if !ok {
			err = utils.NewError(l, span, "GetPlatform: failed to get ID")
			return
		}
		platID = v.(int64)
		success = true
	}
	utils.Debug(l, span, "GetPlatform found '%d': %v", platID, success)
	return
}

func (n Neo) CreatePlatform(ctx context.Context, plat types.Platform) (platID int64, err error) {
	q := `CREATE (n:Platform {arch: "%s", os: "%s", variant: "%s", label: "%s"}) RETURN id(n) AS ID`
	label := fmt.Sprintf("%s/%s", plat.OS, plat.Arch)
	if plat.Variant != "" {
		label = fmt.Sprintf("%s/%s", label, plat.Variant)
	}
	query := fmt.Sprintf(q, plat.Arch, plat.OS, plat.Variant, label)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("failed to create platform: %v", err)
		return
	}
	if result.Next() {
		v, ok := result.Record().Get("ID")
		if !ok {
			err = fmt.Errorf("CreatePlatform: failed to get ID")
			return
		}
		platID = v.(int64)
	}
	return
}

/*
	func (n Neo) LinkPlatformIDToFilterID(ctx context.Context, platID, filterID int64) (err error) {
		q := `MATCH (p:Platform), (f:Filter) WHERE id(p) = %d AND id(f) = %d`
		q += ` MERGE (f)-[:USES]->(p)`
		query := fmt.Sprintf(q, platID, filterID)
		_, err = n.Execute(query)
		if err != nil {
			logrus.Errorf("failed to create link (f:Filter)-[:USES]->(p:Platform): %v", err)
			return
		}
		return
	}

	func (n Neo) LinkPlatformIDToFilter(ctx context.Context, platID int64, filter types.Filter) (err error) {
		q := `MATCH (p:Platform), (f:Filter) WHERE id(p) = %d`
		if filter.Feature != "" {
			q += fmt.Sprintf(" AND f.feature = '%s'", filter.Feature)
		} else {
			q += " AND f.label = 'generic'"
		}
		q += ` MERGE (f)-[:USES]->(p)`
		query := fmt.Sprintf(q, platID)
		session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
		defer session.Close()
		_, err = session.Run(query, map[string]interface{}{})
		if err != nil {
			logrus.Errorf("failed to create link (f:Filter)-[:USES]->(p:Platform): %v", err)
			return
		}
		return
	}

	func (n Neo) InsertPlatformFilter(plat types.Platform, fltr types.Filter) (err error) {
		if plat.ID < 0 {
			return fmt.Errorf("Platform.SqlID is 0")
		}
		if fltr.ID < 0 {
			return fmt.Errorf("Filter.SqlID is 0")
		}
		q := `MATCH (p:Platform), (f:Filter) WHERE id(p) = %d AND id(f) = %d`
		q += " MERGE (f)-[:USES]->(p)"
		query := fmt.Sprintf(q, plat.ID, fltr.ID)
		_, err = n.Execute(query)
		return
	}
*/
func PlatformFromResult(res types.Result) (r types.Platform, err error) {
	err = res.CheckKeys("platformID", "platformOS", "platformArch", "platformVariant")
	if err != nil {
		return
	}
	platformID, _ := res.GetInt64("platformID")
	platformOS, _ := res.GetStr("platformOS")
	platformArch, _ := res.GetStr("platformArch")
	platformVariant, _ := res.GetStr("platformVariant")
	r = types.Platform{
		ID:      platformID,
		Arch:    platformArch,
		OS:      platformOS,
		Variant: platformVariant,
	}
	return
}

func PlatformFromRecord(res neo4j.Result) (p types.Platform, err error) {
	r := res.Record()
	v, ok := r.Get("platformID")
	if !ok {
		err = fmt.Errorf("no configID found in record: %v", r)
		return
	}
	p.ID = v.(int64)
	v, ok = r.Get("platformArch")
	if !ok {
		err = fmt.Errorf("no platformArch found in record: %v", r)
		return
	}
	p.Arch = v.(string)
	v, ok = r.Get("platformOS")
	if !ok {
		err = fmt.Errorf("no platformOS found in record: %v", r)
		return
	}
	p.OS = v.(string)
	v, ok = r.Get("platformVariant")
	if !ok {
		err = fmt.Errorf("no platformVariant found in record: %v", r)
		return
	}
	p.Variant = v.(string)
	return
}
