package neo

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

// TestManipuLayerCreate_RejectsInvalidSelect guards the write-time
// validation added to ManipuLayerCreate: a malformed select expression
// must fail BEFORE any graph node is created, so broken YAML never
// produces a :ManipuLayer node (per the manipulation-apply-select
// spec). The parse guard runs before any n.Execute, so this negative
// path does not depend on a live Neo4j connection.
func TestManipuLayerCreate_RejectsInvalidSelect(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name    string
		selExpr string
		errSub  string
	}{
		{"negative step", "[::-1]", "step must be positive"},
		{"too many colons", "[1:2:3:4]", "not a recognised"},
		{"garbage", "not-a-slice", "not a recognised"},
		{"unbalanced brackets", "[1:2", "unbalanced"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mpl := &types.ManipuLayer{Label: "bad-select", Select: tc.selExpr}
			err := db.ManipuLayerCreate(ctx, mpl)
			require.Error(t, err, "invalid select %q must be rejected", tc.selExpr)
			assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tc.errSub))
			assert.Zero(t, mpl.ID, "no node id should be assigned when validation fails")
		})
	}
}
