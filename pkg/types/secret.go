package types

import (
	"encoding/json"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types/www"
)

type Secret struct {
	SqlID      int64     `json:"-"`
	Name       string    `json:"name"`
	Key        string    `json:"secret_key"`
	IsCopied   bool      `json:"isCopied"`
	CreatedAt  time.Time `json:"created_at"`
	SecretHash string    `json:"-"`
}

func (s Secret) ToResponse() www.Secret {
	return www.Secret{
		ID:         s.SqlID,
		Name:       s.Name,
		Key:        s.Key,
		IsCopied:   s.IsCopied,
		CreatedAt:  s.CreatedAt,
		SecretHash: s.SecretHash,
	}
}

func SecretFromWWW(w www.Secret) (s Secret) {
	s.Key = w.Key
	s.Name = w.Name
	s.CreatedAt = w.CreatedAt
	return
}

func (s Secret) ToBytes() []byte {
	b, err := json.Marshal(s)
	if err != nil {
		logrus.Fatal(err.Error())
		return nil
	}
	return b
}
