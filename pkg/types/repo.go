package types

type Repo struct {
	ID     int64         `json:"id"`
	Repo   string        `json:"repo"`
	Tag    string        `json:"tag"`
	ColTag CollectionTag `json:"-"`
	//Annotations   map[string]string `json:"annotations"`
	ManifestLists []Index `json:"manifest_list"`
}

func NewCollection(repo, tag string) Repo {
	return Repo{
		ID:            -1,
		Repo:          repo,
		Tag:           tag,
		ManifestLists: []Index{},
	}
}
