package www

import (
	"encoding/json"

	"github.com/sirupsen/logrus"
)

type Session struct {
	ID     string `json:"id"`
	Login  string `json:"login"`
	Avatar string `json:"avatar"`
}

func (s Session) ToBytes() []byte {
	b, err := json.Marshal(s)
	if err != nil {
		logrus.Fatal(err.Error())
		return nil
	}
	return b
}
