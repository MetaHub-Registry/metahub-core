package types

type Collection struct {
	ID     int64         `json:"id"`
	Repo   string        `json:"repo"`
	Tag    string        `json:"tag"`
	ColTag CollectionTag `json:"-"`
	//Annotations   map[string]string `json:"annotations"`
	ManifestLists []ManifestList `json:"manifest_list"`
}

func NewCollection(repo, tag string) Collection {
	return Collection{
		ID:            -1,
		Repo:          repo,
		Tag:           tag,
		ManifestLists: []ManifestList{},
	}
}
