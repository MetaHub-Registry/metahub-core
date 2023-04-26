package sqlite

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

/*
	func Test_User_Insert(t *testing.T) {
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
		err = sq.LoginUser("test", "test")
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
		err = sq.LoginUser("test", "test")
		if err != nil {
		t.Fatal(err)
}
		err = sq.LoginUser("test", "FAKE")
		assert.Error(t, err)
		err = sq.LoginUser("FAKE", "FAKE")
		assert.Error(t, err)
		at, err := sq.User.GetAccessToken("test")
		if err != nil {
		t.Fatal(err)
}
		assert.Equal(t, "test", at.Token)
		gotU1, err := sq.User.GetByUsername("test")
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
		err = sq.LoginUser("test", "test")
		assert.Error(t, err)
		err = sq.LoginUser("test", "test2")
		if err != nil {
		t.Fatal(err)
}
		err = sq.User.DeleteByID(int(uID))
		if err != nil {
		t.Fatal(err)
}
	}
*/

func Test_User_Roles(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	bs.AddBootstrap("users")
	//bs.SetSqlite("Desktop/metahub.db")
	br, err := SetupTest(bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs)
	// Test
	testToken, err := br.Get("user:test#bearer")
	if err != nil {
		t.Fatal(err)
	}
	_ = testToken
	adminToken, err := br.Get("user:admin#bearer")
	if err != nil {
		t.Fatal(err)
	}
	_ = adminToken
	_, err = br.Get("user:fake#bearer")
	assert.Error(t, err)

}
