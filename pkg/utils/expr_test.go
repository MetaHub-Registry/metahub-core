package utils

import (
	"testing"

	"github.com/minio/minio-go/v7/pkg/set"
	"github.com/stretchr/testify/assert"
)

func TestEncodeExpression(t *testing.T) {
	q := "foo=bar"
	got, err := EncodeExpression(q)
	if err != nil {
		t.Fatal(err)
	}
	exp := "f666f6f=bar"
	assert.Equal(t, exp, got)
	q1 := "foo=bar && bar=baz"
	got1, err := EncodeExpression(q1)
	if err != nil {
		t.Fatal(err)
	}
	exp1 := "f666f6f=bar && f626172=baz"
	assert.Equal(t, exp1, got1)
	q2 := ""
	_, err = EncodeExpression(q2)
	if err != nil {
		t.Fatal(err)
	}

}

func TestDecode(t *testing.T) {
	k0 := "f666f6f"
	g0, err := DecodeKey(k0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "foo", g0)
}

func TestExprGetLabels(t *testing.T) {
	q := "foo=true"

	got, err := ExprGetLabels(q)
	if err != nil {
		t.Fatal(err)
	}
	exp := set.CreateStringSet("foo")
	assert.Equal(t, exp, got)
	q1 := "foo=true && bar=true"
	got1, err := ExprGetLabels(q1)
	if err != nil {
		t.Fatal(err)
	}
	exp1 := set.CreateStringSet("foo", "bar")
	assert.Equal(t, exp1, got1)
	q2 := "cpu:generic=true"
	got2, err := ExprGetLabels(q2)
	if err != nil {
		t.Fatal(err)
	}
	exp2 := set.CreateStringSet("cpu:generic")
	assert.Equal(t, exp2, got2)
	q3 := "foo=true || bar=true"
	got3, err := ExprGetLabels(q3)
	if err != nil {
		t.Fatal(err)
	}
	exp3 := set.CreateStringSet("foo", "bar")
	assert.Equal(t, exp3, got3)

}
