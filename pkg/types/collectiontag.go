package types

type CollectionTag struct {
	ID  int64  `json:"id"`
	Tag string `json:"tag"`
}

func NewCollectionTag(tag string) CollectionTag {
	return CollectionTag{
		ID:  -1,
		Tag: tag,
	}
}
