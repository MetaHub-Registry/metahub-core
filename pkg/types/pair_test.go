package types

import (
	"testing"
)

func TestProfilePair(t *testing.T) {
	p0 := Profile{
		ID:   1,
		Name: "name",
		Tag:  "tag",
		Rule: "cpu:generic=true",
	}
	c0 := Repo{
		ID:   2,
		Repo: "repo",
	}
	t0 := CollectionTag{
		ID:  3,
		Tag: "tag",
	}
	md1 := Manifest{
		ID: 4,
	}
	pp, err := NewCollectionTagProfilePair(c0, t0, p0)
	if err != nil {
		t.Fatal(err)
	}
	err = pp.AddManifest(md1)
	if err != nil {
		t.Fatal(err)
	}

}
