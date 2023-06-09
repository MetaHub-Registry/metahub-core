package sqlite

import (
	"fmt"
	"testing"
	"time"

	"context"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"golang.org/x/crypto/bcrypt"
)

func TestUserSimple(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	//bs.SetSqlite("Desktop/metahub.db")
	br, err := SetupTest(bs)
	if err != nil {
		t.Fatal(err)
	}
	_ = br
	defer TeardownTest(bs)
	// Test
	_, e := sq.User.getByUsername(ctx, "test")
	assert.Error(t, e)
	if ok, e := sq.UserNameExists(ctx, "test"); e != nil {
		t.Fatal(e)
	} else {
		assert.False(t, ok)
	}
}

func TestUserInsert(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	//bs.SetSqlite("Desktop/metahub.db")
	br, err := SetupTest(bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs)
	// Test
	_ = br
	passwd := "test"
	uname := "user"
	err = sq.LoginUser(ctx, uname, passwd)
	assert.Error(t, err)
	scrt1 := types.Secret{
		CreatedAt: time.Now(),
		IsCopied:  false,
		Key:       passwd,
	}
	u1 := types.User{
		Username: uname,
		Email:    "test@test.de",
		Secrets: []types.Secret{
			scrt1,
		},
	}
	uID, err := sq.InsertUser(ctx, u1)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotEqual(t, int64(-1), uID)
	if ok, e := sq.UserNameExists(ctx, uname); e != nil {
		t.Fatal(e)
	} else {
		assert.True(t, ok)
	}
	err = sq.LoginUser(ctx, uname, passwd)
	if err != nil {
		t.Fatal(err)
	}
	err = sq.LoginUser(ctx, uname, "FAKE")
	assert.Error(t, err)
	err = sq.LoginUser(ctx, "FAKE", "FAKE")
	assert.Error(t, err)
	at, err := sq.User.GetAccessToken("test")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "test", at.Token)
	gotU1, err := sq.GetByUsername(ctx, uname)
	if err != nil {
		t.Fatal(err)
	}
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(gotU1.Secrets[0].SecretHash), []byte(passwd)))
	gotU2, err := sq.GetByEmail(ctx, "test@test.de")
	if err != nil {
		t.Fatal(err)
	}
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(gotU2.Secrets[0].SecretHash), []byte(passwd)))
	gotU3, err := sq.GetOne(ctx, int(uID))
	if err != nil {
		t.Fatal(err)
	}
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(gotU3.Secrets[0].SecretHash), []byte(passwd)))
	gotU3.FirstName = "Max"
	gotU3.LastName = "Mustermann"
	gotU3.Active = 1
	err = gotU3.Update()
	if err != nil {
		t.Fatal(err)
	}
	gotU31, err := sq.GetOne(ctx, int(uID))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "Max", gotU31.FirstName)
	err = gotU31.Delete()
	if err != nil {
		t.Fatal(err)
	}
	_, err = sq.GetOne(ctx, int(uID))
	assert.Error(t, err)
	uID, err = sq.InsertUser(ctx, u1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = sq.GetOne(ctx, int(uID))
	if err != nil {
		t.Fatal(err)
	}
	gotU4, err := sq.GetOne(ctx, int(uID))
	if err != nil {
		t.Fatal(err)
	}
	err = gotU4.ResetPassword("test2")
	if err != nil {
		t.Fatal(err)
	}
	err = sq.LoginUser(ctx, uname, "test")
	assert.Error(t, err)
	err = sq.LoginUser(ctx, uname, "test2")
	if err != nil {
		t.Fatal(err)
	}
	err = sq.User.DeleteByID(int(uID))
	if err != nil {
		t.Fatal(err)
	}
	// ToResponse
	wwwU4 := gotU4.ToResponse()
	assert.Equal(t, wwwU4.GithubUsername, gotU4.UserName)
	assert.Equal(t, 1, len(wwwU4.Secrets))
}

func TestSecretCRD(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	//bs.SetSqlite("Desktop/metahub.db")
	br, err := SetupTest(bs)
	if err != nil {
		t.Fatal(err)
	}
	_ = br
	defer TeardownTest(bs)
	// Test
	uname := "user"
	u1 := types.User{
		Username: uname,
		Email:    "test@test.de",
		Secrets:  []types.Secret{},
	}
	uID, err := sq.InsertUser(ctx, u1)
	if err != nil {
		t.Fatal(err)
	}
	sqlU, e := sq.GetOne(ctx, int(uID))
	if e != nil {
		t.Fatal(e)
	}
	assert.Equal(t, 0, len(sqlU.Secrets))
	scrt1 := types.Secret{
		Name: "name1",
		Key:  "key1",
	}
	scrtID1, err := sq.InsertSecret(ctx, uID, scrt1)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotEqual(t, int64(-1), scrtID1)
	sqlU1, err := sq.GetOne(ctx, int(uID))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(sqlU1.Secrets))
	if e := sq.DelSecret(ctx, int(uID), fmt.Sprintf("%d", scrtID1)); e != nil {
		t.Fatal(e)
	}
	sqlU2, err := sq.GetOne(ctx, int(uID))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 0, len(sqlU2.Secrets))
	scrt2 := types.Secret{
		Name: "name2",
		Key:  "key2",
	}
	_, err = sq.InsertSecret(ctx, uID, scrt2)
	if err != nil {
		t.Fatal(err)
	}
	if e := sq.DelSecretByName(ctx, int(uID), scrt2.Name); e != nil {
		t.Fatal(e)
	}
}
