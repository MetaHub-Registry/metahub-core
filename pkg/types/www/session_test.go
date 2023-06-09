package www

import "testing"

func TestSessionToBytes(t *testing.T) {
	s := Session{
		ID:     "123",
		Login:  "ckniep",
		Avatar: "https://avatars.githubusercontent.com/u/1335026?v=4",
	}
	b := s.ToBytes()
	if string(b) != `{"id":"123","login":"ckniep","avatar":"https://avatars.githubusercontent.com/u/1335026?v=4"}` {
		t.Errorf("Expected %s, got %s", `{"id":"123","login":"ckniep","avatar":"https://avatars.githubusercontent.com/u/1335026?v=4"}`, string(b))
	}
}
