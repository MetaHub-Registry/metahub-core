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
		Username:  "test",
		OidcSub:   "sub-abc-123",
		Email:     "test@test.de",
		FirstName: "Test",
		LastName:  "User",
		AvatarURL: "https://example.com/avatar.png",
	}
	u0 := UserFromWWWUser(wU0, "user")
	assert.Equal(t, wU0.Username, u0.Username)
	assert.Equal(t, wU0.OidcSub, u0.OidcSub)
	assert.Equal(t, wU0.Email, u0.Email)
	assert.Equal(t, wU0.FirstName, u0.FirstName)
	assert.Equal(t, wU0.LastName, u0.LastName)
	assert.Equal(t, wU0.AvatarURL, u0.AvatarURL)
	assert.Equal(t, "user", u0.Role)
	assert.Equal(t, 1, u0.Active)
}

func TestUserFromWWW_EmptyOidcSub(t *testing.T) {
	wU0 := www.User{
		Username: "no-oidc",
		Email:    "no-oidc@test.de",
	}
	u0 := UserFromWWWUser(wU0, "admin")
	assert.Equal(t, "", u0.OidcSub)
	assert.Equal(t, "admin", u0.Role)
	assert.Equal(t, 1, u0.Active)
}
