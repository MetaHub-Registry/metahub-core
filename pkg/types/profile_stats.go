package types

// ProfileStats extends Profile with usage statistics derived from the
// Slot→Index→Manifest graph path.
type ProfileStats struct {
	Profile
	CollectionCount int64 `json:"collection_count"`
	ManifestCount   int64 `json:"manifest_count"`
}
