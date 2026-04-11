package types

import (
	"fmt"
	"time"
)

// Slot is the primary per-triple anchor in the slot-centred graph model.
//
// A Slot represents the intersection of a repo, a tag, and a profile, and owns
// all per-triple state such as the currently-pointed-to Index, push history,
// historical index chain and (eventually) blob-mount edges. The underlying
// :Slot node has a compound UNIQUE constraint on (repo, tag, profile), so there
// is exactly one Slot per logical triple.
//
// Slot replaces the older CollectionTagProfilePair convenience struct in the
// refactor-graph-model-slot-centred change. Unlike the pair, Slot is a first
// class graph citizen: it persists to :Slot nodes with join edges to :Repo,
// :Tag and :Profile, rather than being a transient in-memory aggregation.
type Slot struct {
	ID        int64     `json:"id"`
	Repo      string    `json:"repo"`
	Tag       string    `json:"tag"`
	Profile   string    `json:"profile"`
	CreatedAt time.Time `json:"created_at"`
}

// NewSlot constructs an in-memory Slot for the given triple. The ID is -1
// until the Slot is persisted via a backend Slot{Get,GetCreate,...} helper.
func NewSlot(repo, tag, profile string) Slot {
	return Slot{
		ID:      -1,
		Repo:    repo,
		Tag:     tag,
		Profile: profile,
	}
}

// Key returns a stable string key for the slot triple, suitable for map keys
// and log lines. The format is `repo:tag#profile` and is NOT intended to be
// parsed back into its components - use the individual fields instead.
func (s Slot) Key() string {
	return fmt.Sprintf("%s:%s#%s", s.Repo, s.Tag, s.Profile)
}

// String renders the Slot for log lines in the same shape as the legacy
// CollectionTagProfilePair.String for drop-in replacement.
func (s Slot) String() string {
	return fmt.Sprintf("[%d]%s", s.ID, s.Key())
}
