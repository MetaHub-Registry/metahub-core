package sqlite

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"golang.org/x/crypto/bcrypt"
)

func Test_User_Insert(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	//bs.SetSqlite("Desktop/metahub.db")
	br, err := SetupTest(bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	// Test
	_ = br
	err = sq.LoginUser("test", "test")
	assert.Error(t, err)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("test"), 12)
	assert.NoError(t, err)

	u1 := types.User{
		Username:     "test",
		Email:        "test@test.de",
		PasswordHash: string(hashedPassword),
	}
	uID, err := sq.InsertUser(ctx, u1)
	assert.NoError(t, err)
	assert.NotEqual(t, int64(-1), uID)
	err = sq.LoginUser("test", "test")
	assert.NoError(t, err)
	err = sq.LoginUser("test", "FAKE")
	assert.Error(t, err)
	err = sq.LoginUser("FAKE", "FAKE")
	assert.Error(t, err)
	at, err := sq.User.GetAccessToken("test")
	assert.NoError(t, err)
	assert.Equal(t, "test", at.Token)
	gotU1, err := sq.User.GetByUsername("test")
	assert.NoError(t, err)
	assert.Equal(t, u1.PasswordHash, gotU1.PasswordHash)
	gotU2, err := sq.User.GetByEmail("test@test.de")
	assert.NoError(t, err)
	assert.Equal(t, u1.PasswordHash, gotU2.PasswordHash)
	gotU3, err := sq.User.GetOne(int(uID))
	assert.NoError(t, err)
	assert.Equal(t, u1.PasswordHash, gotU3.PasswordHash)
	gotU3.FirstName = "Max"
	gotU3.LastName = "Mustermann"
	gotU3.Active = 1
	err = gotU3.Update()
	assert.NoError(t, err)
	gotU31, err := sq.User.GetOne(int(uID))
	assert.NoError(t, err)
	assert.Equal(t, "Max", gotU31.FirstName)
	err = gotU31.Delete()
	assert.NoError(t, err)
	_, err = sq.User.GetOne(int(uID))
	assert.Error(t, err)
	uID, err = sq.InsertUser(ctx, u1)
	assert.NoError(t, err)
	_, err = sq.User.GetOne(int(uID))
	assert.NoError(t, err)
	gotU4, err := sq.User.GetOne(int(uID))
	assert.NoError(t, err)
	err = gotU4.ResetPassword("test2")
	assert.NoError(t, err)
	err = sq.LoginUser("test", "test")
	assert.Error(t, err)
	err = sq.LoginUser("test", "test2")
	assert.NoError(t, err)
	err = sq.User.DeleteByID(int(uID))
	assert.NoError(t, err)
}
func Test_User_Roles(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.AddBootstrap("users")
	//bs.SetSqlite("Desktop/metahub.db")
	br, err := SetupTest(bs)
	assert.NoError(t, err)
	defer TeardownTest(bs)
	// Test
	testToken, err := br.Get("user:test#bearer")
	assert.NoError(t, err)
	_ = testToken
	adminToken, err := br.Get("user:admin#bearer")
	assert.NoError(t, err)
	_ = adminToken
	_, err = br.Get("user:fake#bearer")
	assert.Error(t, err)

}
