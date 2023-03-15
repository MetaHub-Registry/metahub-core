package sqlite

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/helper"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"golang.org/x/crypto/bcrypt"
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
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Token        Token     `json:"token"`
}

// / SQlite specific methods
func (s Sqlite) InsertUser(ctx context.Context, u types.User) (userID int64, err error) {
	q := `INSERT INTO users (user_name, email, first_name, last_name, password_hash, user_active, created_at, updated_at, role)`
	q += ` VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	stmt, err := db.Prepare(q)
	if err != nil {
		return
	}
	res, err := stmt.Exec(u.Username, u.Email, u.FirstName, u.LastName, u.PasswordHash, u.Active, time.Now(), time.Now(), u.Role)
	if err != nil {
		return
	}
	userID, err = res.LastInsertId()
	if err != nil {
		return
	}
	u.SqlID = userID
	return
}

func (s Sqlite) LoginUserWithHash(username, pwhash string) (err error) {
	logrus.Debug(">> LoginUser")
	// check login and return token
	u, err := s.User.GetByUsername(username)
	if err != nil {
		return fmt.Errorf("User not found: %s", err.Error())
	}
	match, err := u.PasswordHashMatches(pwhash)
	if err != nil {
		return fmt.Errorf("error when matching passwords")
	}
	if !match {
		return fmt.Errorf("passwordhash does not match")
	}
	return
}

func (s Sqlite) LoginUserScope(username, password string, req types.Request) (status int, role string, pl helper.ErrorPayload, err error) {
	// check login and return token
	u, err := s.User.GetByUsername(username)
	if err != nil {
		pl = helper.NewErrPayload(
			"UNAUTHORIZED",
			"User not found",
			helper.NewDetailFromReq(req),
		)
		return http.StatusNotFound, role, pl, nil
	}
	match, err := u.PasswordMatches(password)
	if err != nil {
		logrus.Error(err)
		return http.StatusInternalServerError, role, pl, err
	}
	if !match {
		pl = helper.NewErrPayload(
			"UNAUTHORIZED",
			"wrong credentials",
			helper.NewDetailFromReq(req),
		)
		logrus.Error("wrong credentials")
		return http.StatusUnauthorized, role, pl, nil
	}
	switch req.Method {
	case "PUT", "POST", "DELETE", "PATCH":
		if u.Role != "admin" {
			pl = helper.NewErrPayload(
				"UNAUTHORIZED",
				"wrong privileges",
				helper.NewDetailFromReq(req),
			)
			return http.StatusUnauthorized, role, pl, nil
		}
	}
	return http.StatusOK, u.Role, pl, nil
}
func (s Sqlite) LoginUser(username, password string) (err error) {
	logrus.Debug(">> LoginUser")
	// check login and return token
	u, err := s.User.GetByUsername(username)
	if err != nil {
		return fmt.Errorf("User not found: %s", err.Error())
	}
	match, err := u.PasswordMatches(password)
	if err != nil {
		return fmt.Errorf("error when matching passwords")
	}
	if !match {
		return fmt.Errorf("password does not match")
	}
	return
}

// / User specific methods
func (u User) CreateTables() (err error) {
	tableName := "users"
	q := fmt.Sprintf(`PRAGMA table_info(%s)`, tableName)
	rows, err := db.Query(q)
	if err != nil {
		logrus.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		//logrus.Infof("Table '%s' already exists!", tableName)
		return
	}
	s := `CREATE TABLE IF NOT EXISTS %s (
        id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		"user_name" TEXT,
		"email" TEXT,
		"first_name" TEXT,
		"last_name" TEXT,
		"role" TEXT,
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
	logrus.Debugf("Table '%s' created successfully!", tableName)
	return
}

// / That needs some improvement
func (u *User) GetAccessToken(token string) (at *types.AccessToken, err error) {

	at = &types.AccessToken{
		Token: token,
	}
	return at, nil
}

// GetAll returns a slice of all users, sorted by last name
/*func (u *User) GetAll() ([]*User, error) {
	ctx := context.Background()

	query := `select id, email, first_name, last_name, password_hash, user_active, created_at, updated_at,
	CASE
		WHEN (SELECT COUNT(id) FROM tokens t WHERE user_id = users.id AND t.expiry > NOW()) > 0 THEN 1
		ELSE 0
    END AS has_token
	FROM users ORDER BY last_name`
	logrus.Warn(query)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User

	for rows.Next() {
		var user User
		err := rows.Scan(
			&user.ID,
			&user.Email,
			&user.FirstName,
			&user.LastName,
			&user.PasswordHash,
			&user.Active,
			&user.CreatedAt,
			&user.UpdatedAt,
			&user.Token.ID,
		)
		if err != nil {
			return nil, err
		}

		users = append(users, &user)
	}

	return users, nil
}*/

// GetByEmail returns one user by email
func (u *User) GetByUsername(username string) (*User, error) {
	ctx := context.Background()

	query := `SELECT id, user_name, email, first_name, last_name, password_hash, user_active, created_at, role FROM users WHERE user_name = $1`

	var user User
	row := db.QueryRowContext(ctx, query, username)

	err := row.Scan(
		&user.ID,
		&user.UserName,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.PasswordHash,
		&user.Active,
		&user.CreatedAt,
		&user.Role,
	)

	if err != nil {
		return nil, err
	}

	return &user, nil
}

// GetByEmail returns one user by email
func (u *User) GetByEmail(email string) (*User, error) {
	ctx := context.Background()

	query := `SELECT id, user_name, email, first_name, last_name, password_hash, user_active, created_at, updated_at ,role FROM users WHERE email = $1`

	var user User
	row := db.QueryRowContext(ctx, query, email)

	err := row.Scan(
		&user.ID,
		&user.UserName,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.PasswordHash,
		&user.Active,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.Role,
	)

	if err != nil {
		return nil, err
	}

	return &user, nil
}

// GetOne returns one user by id
func (u *User) GetOne(id int) (*User, error) {
	ctx := context.Background()

	query := `select id, user_name, email, first_name, last_name, password_hash, user_active, created_at, updated_at, role from users where id = $1`

	var user User
	row := db.QueryRowContext(ctx, query, id)

	err := row.Scan(
		&user.ID,
		&user.UserName,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.PasswordHash,
		&user.Active,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.Role,
	)

	if err != nil {
		return nil, err
	}

	return &user, nil
}

// Update updates one user in the database, using the information
// stored in the receiver u
func (u *User) Update() error {
	ctx := context.Background()

	// TODO: We do not allow to update the username ,right?
	stmt := `update users set
		email = $1,
		first_name = $2,
		last_name = $3,
		user_active = $4,
		updated_at = $5
		where id = $6
	`

	_, err := db.ExecContext(ctx, stmt,
		u.Email,
		u.FirstName,
		u.LastName,
		u.Active,
		time.Now(),
		u.ID,
	)

	if err != nil {
		return err
	}

	return nil
}

// Delete deletes one user from the datbase, by ID
func (u *User) Delete() error {
	ctx := context.Background()

	stmt := `delete from users where id = $1`

	_, err := db.ExecContext(ctx, stmt, u.ID)
	if err != nil {
		return err
	}

	return nil
}

// Delete deletes one user from the datbase, by ID
func (u *User) DeleteByID(id int) error {
	ctx := context.Background()

	stmt := `delete from users where id = $1`
	_, err := db.ExecContext(ctx, stmt, id)
	if err != nil {
		return err
	}

	return nil
}

// ResetPassword is the method we will use to change a user's password.
func (u *User) ResetPassword(password string) error {
	ctx := context.Background()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}

	stmt := `update users set password_hash = $1 where id = $2`
	_, err = db.ExecContext(ctx, stmt, hashedPassword, u.ID)
	if err != nil {
		return err
	}

	return nil
}

// PasswordMatches uses Go's bcrypt package to compare a user supplied password
// with the hash we have stored for a given user in the database. If the password
// and hash match, we return true; otherwise, we return false.
func (u *User) PasswordMatches(plainText string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(plainText))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			// invalid password
			return false, nil
		default:
			return false, err
		}
	}

	return true, nil
}

func (u *User) PasswordHashMatches(passwordHash string) (bool, error) {
	if u.PasswordHash == passwordHash {
		return true, nil
	}
	return false, nil
}
