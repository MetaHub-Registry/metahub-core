package types

import (
	"time"

	"github.com/metahub-registry/metahub-core/pkg/types/www"
)

type Secret struct {
	SqlID      int64     `json:"-"`
	Name       string    `json:"name"`
	Key        string    `json:"secret-key"`
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
