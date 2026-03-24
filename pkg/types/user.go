package types

import (
	"github.com/metahub-registry/metahub-core/pkg/types/www"
)

type User struct {
	SqlID      int64  `json:"sql_id"`
	Username   string `yaml:"username"`
	OidcSub    string `yaml:"oidc_sub"`
	Email      string `yaml:"email"`
	Active     int    `yaml:"active"`
	AvatarURL  string `yaml:"avatar_url"`
	FirstName  string `yaml:"first_name,omitempty"`
	LastName   string `yaml:"last_name,omitempty"`
	Role       string `yaml:"role"`
	IsLoggedIn int    `yaml:"is_logged_in"`
	Secrets    []Secret
}

func UserFromWWWUser(w www.User, role string) (u User) {
	u.Username = w.Username
	u.OidcSub = w.OidcSub
	u.Role = role
	u.Email = w.Email
	u.FirstName = w.FirstName
	u.LastName = w.LastName
	u.Active = 1
	u.AvatarURL = w.AvatarURL
	return
}
