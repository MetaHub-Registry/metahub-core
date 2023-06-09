package www

import (
	"encoding/json"
	"time"

	"github.com/sirupsen/logrus"
)

type Secret struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Key        string    `json:"secret_key"`
	IsCopied   bool      `json:"is_copied"`
	CreatedAt  time.Time `json:"created_at"`
	SecretHash string    `json:"-"`
}

func (scrt Secret) ToBytes() []byte {
	b, err := json.Marshal(scrt)
	if err != nil {
		logrus.Fatal(err.Error())
		return nil
	}
	return b
}
