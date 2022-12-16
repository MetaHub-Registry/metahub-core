package types

type Collection struct {
	ID            int64          `json:"id"`
	Repo          string         `json:"repo"`
	Tag           string         `json:"tag"`
	ColTag        CollectionTag  `json:"-"`
	ManifestLists []ManifestList `json:"manifest_list"`
}

func NewCollection() Collection {
	return Collection{
		ID:            -1,
		ManifestLists: []ManifestList{},
	}
}
