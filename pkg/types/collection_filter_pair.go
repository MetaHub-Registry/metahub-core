package types

type CollectionFilterPair struct {
	Repo Repo
	Filter     Filter
	Manifests  []Manifest
}
