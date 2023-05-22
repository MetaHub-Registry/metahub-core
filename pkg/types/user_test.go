package types

import (
	"testing"
	"time"

	"github.com/metahub-registry/metahub-core/pkg/types/www"
	"gotest.tools/v3/assert"
)

func TestSecretToResponse(t *testing.T) {
	s0 := Secret{
		SqlID:      1,
		Key:        "test",
		IsCopied:   false,
		CreatedAt:  time.Now(),
		SecretHash: "hash",
	}
	r1 := s0.ToResponse()
	assert.Equal(t, s0.SqlID, r1.ID)

}

func TestUserFromWWW(t *testing.T) {
	wU0 := www.User{
		GithubUsername: "test",
		Email:          "test@test.de",
		FirstName:      "Test",
		LastName:       "User",
	}
	u0 := UserFromWWWUser(wU0, "user")
	assert.Equal(t, wU0.GithubUsername, u0.Username)
	assert.Equal(t, wU0.Email, u0.Email)
	assert.Equal(t, wU0.FirstName, u0.FirstName)
	assert.Equal(t, wU0.LastName, u0.LastName)
	assert.Equal(t, "user", u0.Role)

}
