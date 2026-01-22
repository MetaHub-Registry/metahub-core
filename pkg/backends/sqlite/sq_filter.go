package sqlite

/*
func (s Sqlite) FilterInsert(ctx context.Context, f types.Filter) (filterID int64, err error) {
	filterID, success, err := s.Filter.GetFilter(f)
	if err != nil {
		return
	}
	if !success {
		err = s.Filter.InsertFilter(&f)
		if err != nil {
			return
		}
		filterID = f.ID
	}
	return
}

func (s Sqlite) FilterUpdatePlatform(ctx context.Context, plat types.Platform) (err error) {
	return s.UpdatePlatformFilter(plat)
}
*/
