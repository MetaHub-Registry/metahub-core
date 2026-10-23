package www

import (
	"bytes"
	"testing"
	"time"
)

func TestSecretToBytes(t *testing.T) {
	now := time.Now()
	s0 := Secret{
		ID:        1,
		Name:      "foo",
		Key:       "bar",
		IsCopied:  true,
		CreatedAt: now,
	}
	b0 := s0.ToBytes()
	if bytes.Contains(b0, []byte(`"id"`)) {
		t.Fatalf("Secret.ToBytes() leaked the storage ID: %s", b0)
	}
	s1 := Secret{
		ID:        1,
		Name:      "foo",
		Key:       "bar",
		IsCopied:  true,
		CreatedAt: now,
	}
	b1 := s1.ToBytes()
	if string(b0) != string(b1) {
		t.Errorf("Secret.ToBytes() = %v, want %v", string(b0), string(b1))
	}
}
