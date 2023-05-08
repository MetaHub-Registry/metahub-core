package sqlite

import (
	"testing"

	"context"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"golang.org/x/crypto/bcrypt"
)

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
	err = sq.LoginUser(ctx, "test", "test")
	assert.Error(t, err)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("test"), 12)
	if err != nil {
		t.Fatal(err)
	}

	u1 := types.User{
		Username:     "test",
		Email:        "test@test.de",
		PasswordHash: string(hashedPassword),
	}
	uID, err := sq.InsertUser(ctx, u1)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotEqual(t, int64(-1), uID)
	err = sq.LoginUser(ctx, "test", "test")
	if err != nil {
		t.Fatal(err)
	}
	err = sq.LoginUser(ctx, "test", "FAKE")
	assert.Error(t, err)
	err = sq.LoginUser(ctx, "FAKE", "FAKE")
	assert.Error(t, err)
	at, err := sq.User.GetAccessToken("test")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "test", at.Token)
	gotU1, err := sq.User.GetByUsername(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, u1.PasswordHash, gotU1.PasswordHash)
	gotU2, err := sq.User.GetByEmail("test@test.de")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, u1.PasswordHash, gotU2.PasswordHash)
	gotU3, err := sq.User.GetOne(int(uID))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, u1.PasswordHash, gotU3.PasswordHash)
	gotU3.FirstName = "Max"
	gotU3.LastName = "Mustermann"
	gotU3.Active = 1
	err = gotU3.Update()
	if err != nil {
		t.Fatal(err)
	}
	gotU31, err := sq.User.GetOne(int(uID))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "Max", gotU31.FirstName)
	err = gotU31.Delete()
	if err != nil {
		t.Fatal(err)
	}
	_, err = sq.User.GetOne(int(uID))
	assert.Error(t, err)
	uID, err = sq.InsertUser(ctx, u1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = sq.User.GetOne(int(uID))
	if err != nil {
		t.Fatal(err)
	}
	gotU4, err := sq.User.GetOne(int(uID))
	if err != nil {
		t.Fatal(err)
	}
	err = gotU4.ResetPassword("test2")
	if err != nil {
		t.Fatal(err)
	}
	err = sq.LoginUser(ctx, "test", "test")
	assert.Error(t, err)
	err = sq.LoginUser(ctx, "test", "test2")
	if err != nil {
		t.Fatal(err)
	}
	err = sq.User.DeleteByID(int(uID))
	if err != nil {
		t.Fatal(err)
	}
}
