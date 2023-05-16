package utils

import "testing"

func TestBootstrap(t *testing.T) {
	br := NewBootstrapResult()
	br.Add("foo", "bar")
	br.Add("int64", "64")
	br.Add("int", "42")
	if res, e := br.GetIntRegEx("^int$"); e != nil {
		t.Fatal(e)
	} else {
		r, ok := res["int"]
		if !ok {
			t.Fatal("int not found")
		}
		if r != 42 {
			t.Fatal("int not 42")
		}
	}
	if res, e := br.GetInt64RegEx("^int64$"); e != nil {
		t.Fatal(e)
	} else {
		r, ok := res["int64"]
		if !ok {
			t.Fatal("int64 not found")
		}
		if r != int64(64) {
			t.Fatal("int64 not 64")
		}
	}
}
