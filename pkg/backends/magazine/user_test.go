package magazine

import (
	"context"
	"testing"

	"github.com/metahub-registry/metahub-core/pkg/types"
)

var ctx = context.Background()

func TestUserInsert(t *testing.T) {
	user := types.User{
		Username:     "test",
		PasswordHash: "test",
	}
	err := mag.InsertUser(ctx, user)
	if err != nil {
		t.Fatal(err)
	}

}
