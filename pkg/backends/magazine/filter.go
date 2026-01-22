package magazine

/*
func (mag Magazine) InsertFilter(ctx context.Context, filter types.Filter) (filterID int64, err error) {
	filterID, err = mag.Database.FilterInsert(ctx, filter)
	if err != nil {
		return
	}
	// Iterate through Annotations (and other filte targets?) to connect rules
	for _, r := range filter.Rules {
		rId, found, e := mag.Database.RuleGet(ctx, r)
		if e != nil {
			err = errors.Wrap(e, "mag.Database.RuleGet")
			logrus.Error(err)
			return filterID, err
		}
		if !found {
			err = fmt.Errorf("rule not found: %v", r)
			return
		}
		r.ID = rId
		switch r.Type {
		case types.RuleTypeAnnotation:
			// Insert Annotation
			err = mag.Database.RuleEvalAnnotationRule(ctx, r)
			if err != nil {
				logrus.Error(err)
			}
		}
	}
	return
}
*/
