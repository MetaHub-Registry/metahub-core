package neo

/*
func (n Neo) FilterInsert(ctx context.Context, f types.Filter) (filterID int64, err error) {
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

func (n Neo) GetFilter(f types.Filter) (filterID int64, found bool, err error) {
	q := `MATCH (n:Filter) WHERE n.name = "%s" RETURN id(n) AS ID`
	query := fmt.Sprintf(q, f.Name)
	res, found, err := n.QueryOne(query)
	if err != nil {
		logrus.Errorf(errFailedRunQueryFmt, err)
		return
	}
	if !found {
		return
	}
	filterID, err = res.GetInt64("ID")
	return
}

func (n Neo) FilterGetByID(id int64) (f types.Filter, found bool, err error) {
	q := `MATCH (n:Filter) WHERE id(n) = %d RETURN id(n) AS filterID, n.name AS filterName`
	query := fmt.Sprintf(q, id)
	res, qFound, err := n.QueryOne(query)
	if err != nil {
		logrus.Error(err)
		return
	}
	if !qFound {
		return
	}
	f, err = n.FilterFromResult(res)
	if err != nil {
		logrus.Error(err)
		return
	}
	found = true
	return
}

func (n Neo) CreateFilter(f types.Filter) (filterID int64, err error) {
	ctx := context.Background()
	q := `CREATE (n:Filter {name: "%s"}) RETURN id(n) AS filterID`
	query := fmt.Sprintf(q, f.Name)
	res, err := n.Execute(query)
	if err != nil {
		logrus.Error(err)
		return
	}
	filterID, err = res.GetInt64("filterID")
	for _, r := range f.Rules {
		rID, err := n.RuleGetCreate(ctx, r)
		if err != nil {
			logrus.Error(err)
			continue
		}
		r.ID = rID
		err = n.FilterLinkToRule(ctx, filterID, r)
		if err != nil {
			logrus.Error(err)
			continue
		}
	}
	return
}

func (n Neo) FilterLinkToRule(ctx context.Context, filterID int64, r types.Rule) (err error) {
	q := `MATCH (f:Filter), (r:Rule) WHERE id(f) = %d AND id(r) = %d MERGE (f)-[:HAS_RULE]->(r)`
	query := fmt.Sprintf(q, filterID, r.ID)
	_, err = n.Execute(query)
	if err != nil {
		logrus.Error(err)
		return
	}
	return
}

/*
func (n Neo) FilterGetByLabel(label string) (f types.Filter, found bool, err error) {
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

func (n Neo) FilterGetByFeature(feat string) (f types.Filter, found bool, err error) {
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

func (n Neo) FilterGetByFeatureRegex(features string) (filters []types.Filter, err error) {
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
func (n Neo) FilterUpdatePlatform(ctx context.Context, plat types.Platform) (err error) {
	/// Does the platform have a features?
	if plat.Features == "" {
		var fID int64
		// No features, check for generic filter
		f, found, e := n.FilterGetByLabel("generic")
		if e != nil {
			logrus.Error(e)
			return e
		}
		if !found {
			fID, err = n.FilterInsert(ctx, types.Filter{Label: "generic"})
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
		f, found, err := n.FilterGetByFeature(plat.Features)
		if err != nil {
			logrus.Error(err)
			return err
		}
		if !found {
			/// create filter
			f.ID, err = n.FilterInsert(ctx, types.Filter{Label: plat.Features, Feature: plat.Features})
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
*/

/*
func (n Neo) FilterFromResult(res types.Result) (f types.Filter, err error) {
	ctx := context.Background()
	err = res.CheckKeys("filterID", "filterName")
	if err != nil {
		return
	}
	id, _ := res.GetInt64("filterID")
	name, _ := res.GetStr("filterName")
	f = types.Filter{
		ID:   id,
		Name: name,
	}
	f.Rules, err = n.RulesGetByFilterID(ctx, id)
	if err != nil {
		logrus.Error(err)
		return
	}
	return
}


func (n Neo) ProfileManifestPairGet() (fmdps types.ProfileManifestPairs, err error) {
	q := `MATCH (md:Manifest)-[:HAS_ANNOTATION]->(Annotation)<-[:TRUE]-(Rule)<-[:HAS_RULE]->(p:Profile)`
	q += ` RETURN DISTINCT`
	logrus.Fatal("Check manifestData return values!")
	q += ` ` + manifestDataReturn + `,`
	q += ` md.size AS manifestSize,md.label AS manifestLabel,`
	q += ` id(p) AS profileID, p.name AS profileName, p.tag AS profileTag`
	res, err := n.Query(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	pmdps = types.NewProfileManifestPairs()
	for _, r := range res {
		p, err := n.ProfileFromResult(r)
		if err != nil {
			logrus.Error(err)
			continue
		}
		md, err := n.ManifestFromResult(r)
		if err != nil {
			logrus.Error(err)
			continue
		}
		pmdp := types.ProfileManifestPair{
			Profile:      p,
			Manifest: md,
		}
		_ = pmdp
		//fmdps.Pairs = append(fmdps.Pairs, fmdp)
	}
	_ = pmdps
	return
}
*/
