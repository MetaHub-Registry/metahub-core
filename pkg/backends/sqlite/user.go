package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
)

// User is the stucture which holds one user from the database. Note
// that it embeds a token type.
type User struct {
	ID           int       `json:"id"`
	UserName     string    `json:"user_name"`
	Email        string    `json:"email"`
	FirstName    string    `json:"first_name,omitempty"`
	LastName     string    `json:"last_name,omitempty"`
	PasswordHash string    `json:"password_hash"`
	Active       int       `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Token        Token     `json:"token"`
}

func (u User) CreateTables() (err error) {
	logrus.Warn("CreateTables")
	tableName := "users"
	q := fmt.Sprintf(`PRAGMA table_info(%s)`, tableName)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		logrus.Infof("Table '%s' already exists!", tableName)
		return
	}
	s := `CREATE TABLE IF NOT EXISTS %s (
        id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		"user_name" TEXT,
		"email" TEXT,
		"first_name" TEXT,
		"last_name" TEXT,
		"password_hash" TEXT,
		"user_active" INTEGER,
		"created_at" TIMESTAMP,
		"updated_at" TIMESTAMP
		);`
	stmt := fmt.Sprintf(s, tableName)
	query, err := db.Prepare(stmt)
	if err != nil {
		logrus.Fatal(err)
	}
	_, err = query.Exec()
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Infof("Table '%s' created successfully!", tableName)
	return
}

func (u User) CreateViews() (err error) {
	return
}

/// SQLITE

func (s Sqlite) InsertUser(ctx context.Context, u types.User) (userID int64, err error) {
	q := `INSERT INTO users (user_name, email, first_name, last_name, password_hash, user_active, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
	stmt, err := db.Prepare(q)
	if err != nil {
		logrus.Error(err)
		return
	}
	res, err := stmt.Exec(u.Username, u.Email, u.FirstName, u.LastName, u.PasswordHash, u.Active, time.Now())
	if err != nil {
		logrus.Error(err)
		return
	}
	userID, err = res.LastInsertId()
	if err != nil {
		logrus.Error(err)
		return
	}
	u.SqlID = userID
	return
}
