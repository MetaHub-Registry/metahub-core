package types

type CollectionListing struct {
	ID            int64                   `json:"id"`
	Repo          string                  `json:"repo"`
	Tag           string                  `json:"tag"`
	ColTag        CollectionTag           `json:"-"`
	Annotations   map[string]string       `json:"annotations"`
	Manifests     map[string]Manifest `json:"manifests"`
	ManifestCount int64                   `json:"manifest_count"`
}

func NewCollectionListing(repo, tag string) CollectionListing {
	ct := NewCollectionTag(tag)
	return CollectionListing{
		ID:            -1,
		Tag:           tag,
		Repo:          repo,
		ColTag:        ct,
		Manifests:     map[string]Manifest{},
		ManifestCount: 0,
	}
}
