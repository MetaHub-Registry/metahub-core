package www

import (
	"encoding/json"

	"github.com/sirupsen/logrus"
)

type User struct {
	ID        int    `json:"id"`
	Username  string `json:"username"`
	OidcSub   string `json:"oidc_sub"`
	AvatarURL string `json:"avatar_url"`
	Email     string `json:"email"`
	Active    int    `json:"active"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Role      string `json:"role"`
	Secrets   []Secret
}

func (u User) ToBytes() []byte {
	b, err := json.Marshal(u)
	if err != nil {
		logrus.Fatal(err.Error())
		return nil
	}
	return b
}
