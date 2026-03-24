package www

import (
	"encoding/json"
	"testing"
)

func TestUserToBytes(t *testing.T) {
	u := User{
		ID:        42,
		Username:  "alice",
		OidcSub:   "sub-alice-001",
		AvatarURL: "https://example.com/alice.png",
		Email:     "alice@example.com",
		Active:    1,
		FirstName: "Alice",
		LastName:  "Smith",
		Role:      "admin",
	}
	b := u.ToBytes()
	if len(b) == 0 {
		t.Fatal("ToBytes() returned empty bytes")
	}
	var decoded User
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("ToBytes() produced invalid JSON: %v", err)
	}
	if decoded.Username != u.Username {
		t.Errorf("Username: got %q, want %q", decoded.Username, u.Username)
	}
	if decoded.OidcSub != u.OidcSub {
		t.Errorf("OidcSub: got %q, want %q", decoded.OidcSub, u.OidcSub)
	}
	if decoded.Email != u.Email {
		t.Errorf("Email: got %q, want %q", decoded.Email, u.Email)
	}
	if decoded.Role != u.Role {
		t.Errorf("Role: got %q, want %q", decoded.Role, u.Role)
	}
}

func TestUserToBytes_Empty(t *testing.T) {
	u := User{}
	b := u.ToBytes()
	if len(b) == 0 {
		t.Fatal("ToBytes() returned empty bytes for zero-value User")
	}
	var decoded User
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("ToBytes() produced invalid JSON: %v", err)
	}
}
