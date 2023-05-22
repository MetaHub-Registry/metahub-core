package types

import (
	"fmt"

	"github.com/google/go-github/github"
	"github.com/metahub-registry/metahub-core/pkg/types/www"
)

type User struct {
	SqlID     int64  `json:"sql_id"`
	Username  string `yaml:"username"`
	GithubID  string `yaml:"github_id"`
	Email     string `yaml:"email"`
	Active    int    `yaml:"active"`
	AvatarURL string `yaml:"avatar_url"`
	FirstName string `yaml:"first_name,omitempty"`
	LastName  string `yaml:"last_name,omitempty"`
	Role      string `yaml:"role"`
	Secrets   []Secret
}

func UserFromWWWUser(w www.User, role string) (u User) {
	u.Username = w.GithubUsername
	u.GithubID = w.GithubUserID
	u.Role = role
	u.Email = w.Email
	u.FirstName = w.FirstName
	u.LastName = w.LastName
	u.Active = 1
	u.AvatarURL = w.AvatarURL
	return
}

func UserFromGitHubUser(u *github.User) User {
	return User{
		Username:  *u.Login,
		GithubID:  fmt.Sprintf("%d", *u.ID),
		Active:    1,
		AvatarURL: *u.AvatarURL,
	}
}
