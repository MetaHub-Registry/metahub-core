package magazine

import (
	"context"
	"testing"

	"github.com/metahub-registry/metahub-core/pkg/types"
)

var ctx = context.Background()

func Test_User_Insert(t *testing.T) {
	user := types.User{
		Username:     "test",
		PasswordHash: "test",
	}
	err := mag.InsertUser(ctx, user)
	if err != nil {
		t.Fatal(err)
	}

}
