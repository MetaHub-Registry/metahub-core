package neo

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

func (n Neo) Filter_Insert(ctx context.Context, f types.Filter) (filterID int64, err error) {
	filterID, success, err := n.GetFilter(f)
	if err != nil {
		return
	}
	if !success {
		filterID, err = n.CreateFilter(f)
		if err != nil {
			return
		}
	}
	return
}

func (n Neo) GetFilter(f types.Filter) (filterID int64, success bool, err error) {
	q := `MATCH (n:Filter) WHERE n.label = "%s" RETURN id(n) AS ID`
	query := fmt.Sprintf(q, f.Label)
	session := n.neoDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()
	result, err := session.Run(query, map[string]interface{}{})
	if err != nil {
		logrus.Errorf("Failed to run query: %s", err)
		return
	}
	if result.Next() {
		v, ok := result.Record().Get("ID")
		if !ok {
			err = fmt.Errorf("GetFilter: failed to get ID")
			return
		}
		filterID = v.(int64)
		success = true
	}
	return
}

func (n Neo) Filter_Get_ByID(id int64) (f types.Filter, found bool, err error) {
	q := `MATCH (n:Filter) WHERE id(n) = %d RETURN id(n) AS filterID, n.label AS filterLabel, n.feature AS filterFeature LIMIT 1`
	query := fmt.Sprintf(q, id)
	res, qFound, err := n.QueryOne(query)
	if err != nil {
		logrus.Error(err)
		return
	}
	if !qFound {
		return
	}
	f, err = FilterFromResult(res)
	if err != nil {
		logrus.Error(err)
		return
	}
	found = true
	return
}

func (n Neo) Filter_Get_ByLabel(label string) (f types.Filter, found bool, err error) {
	q := `MATCH (n:Filter) WHERE n.label = "%s" RETURN id(n) AS filterID, n.label AS filterLabel, n.feature AS filterFeature LIMIT 1`
	query := fmt.Sprintf(q, label)
	res, qFound, err := n.QueryOne(query)
	if err != nil {
		logrus.Error(err)
		return
	}
	if !qFound {
		return
	}
	f, err = FilterFromResult(res)
	if err != nil {
		logrus.Error(err)
		return
	}
	found = true
	return
}
func (n Neo) Filter_Get_ByFeature(feat string) (f types.Filter, found bool, err error) {
	q := `MATCH (n:Filter) WHERE n.feature = "%s" RETURN id(n) AS filterID, n.label AS filterLabel, n.feature AS filterFeature LIMIT 1`
	query := fmt.Sprintf(q, feat)
	res, qFound, err := n.QueryOne(query)
	if err != nil {
		logrus.Error(err)
		return
	}
	if !qFound {
		return
	}
	f, err = FilterFromResult(res)
	if err != nil {
		logrus.Error(err)
		return
	}
	found = true
	return
}

func (n Neo) CreateFilter(f types.Filter) (filterID int64, err error) {
	q := `CREATE (n:Filter {label: "%s", feature: "%s"}) RETURN id(n) AS ID`
	query := fmt.Sprintf(q, f.Label, f.Feature)
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
			err = fmt.Errorf("CreateFilter: failed to get ID")
			return
		}
		filterID = v.(int64)
	}
	return
}

func (n Neo) Filter_Get_ByFeatureRegex(features string) (filters []types.Filter, err error) {
	q := `MATCH (n:Filter) WHERE n.feature =~ "%s" RETURN id(n) AS filterID, n.label AS filterLabel, n.feature AS filterFeature`
	query := fmt.Sprintf(q, features)
	res, err := n.Query(query)
	if err != nil {
		logrus.Error(err)
		return
	}
	for _, r := range res {
		f, err := FilterFromResult(r)
		if err != nil {
			logrus.Error(err)
			continue
		}
		filters = append(filters, f)
	}
	return
}

// UpdatePlatformFilter checks if an existing filter matches and create a link if need be
func (n Neo) Filter_UpdatePlatform(ctx context.Context, plat types.Platform) (err error) {
	/// Does the platform have a features?
	if plat.Features == "" {
		var fID int64
		// No features, check for generic filter
		f, found, e := n.Filter_Get_ByLabel("generic")
		if e != nil {
			logrus.Error(e)
			return e
		}
		if !found {
			fID, err = n.Filter_Insert(ctx, types.Filter{Label: "generic"})
			if err != nil {
				logrus.Error(err)
				return err
			}
		} else {
			fID = f.ID
		}
		// UpsertCreate link
		err = n.LinkPlatformIDToFilterID(ctx, plat.ID, fID)
		return err
	} else {
		// check if a filter already exists for the features
		f, found, err := n.Filter_Get_ByFeature(plat.Features)
		if err != nil {
			logrus.Error(err)
			return err
		}
		if !found {
			/// create filter
			f.ID, err = n.Filter_Insert(ctx, types.Filter{Label: plat.Features, Feature: plat.Features})
			if err != nil {
				logrus.Error(err)
				return err
			}
		}
		// UpsertCreate link
		err = n.LinkPlatformIDToFilterID(ctx, plat.ID, f.ID)
		return err
	}
}

func FilterFromResult(res Result) (f types.Filter, err error) {
	err = res.CheckKeys("filterID", "filterLabel", "filterFeature")
	if err != nil {
		return
	}
	id, _ := res.GetInt64("filterID")
	label, _ := res.GetStr("filterLabel")
	feat, _ := res.GetStr("filterFeature")

	f = types.Filter{
		ID:      id,
		Label:   label,
		Feature: feat,
	}
	return
}
