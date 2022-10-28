package magazine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

var ctx = context.Background()

func Test_User_Insert(t *testing.T) {
	user := types.User{
		Username:     "test",
		PasswordHash: "test",
	}
	err := mag.InsertUser(ctx, user)
	assert.NoError(t, err)

}
