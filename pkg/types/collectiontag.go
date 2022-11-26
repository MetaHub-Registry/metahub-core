package types

type CollectionTag struct {
	NeoID int64  `json:"neo_id"`
	SqlID int64  `json:"sql_id"`
	Tag   string `json:"tag"`
}

func NewCollectionTag(tag string) CollectionTag {
	return CollectionTag{
		SqlID: -1,
		NeoID: -1,
		Tag:   tag,
	}
}
