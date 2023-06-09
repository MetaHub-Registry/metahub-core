package magazine

import (
	"context"
	"testing"

	"github.com/metahub-registry/metahub-core/pkg/types"
)

func TestUserInsert(t *testing.T) {
	ctx := context.Background()
	user := types.User{
		Username: "test",
	}
	err := mag.InsertUser(ctx, user)
	if err != nil {
		t.Fatal(err)
	}

}
