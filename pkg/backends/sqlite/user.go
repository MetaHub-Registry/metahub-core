package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/helper"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/types/www"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"golang.org/x/crypto/bcrypt"
)

const (
	pkgNameUser = "sqlite/user.go"
)

// Token holds an authentication token for a user session.
type Token struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Email     string    `json:"email"`
	Token     string    `json:"token"`
	TokenHash []byte    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Expiry    time.Time `json:"expiry"`
}

// User is the structure which holds one user from the database. Note
// that it embeds a token type.
type User struct {
	ID         int            `json:"id"`
	OidcSub    string         `json:"oidc_sub"`
	AvatarURL  string         `json:"avatar_url"`
	UserName   string         `json:"user_name"`
	Email      string         `json:"email"`
	FirstName  string         `json:"first_name,omitempty"`
	LastName   string         `json:"last_name,omitempty"`
	Active     int            `json:"active"`
	Role       string         `json:"role"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	Token      Token          `json:"token"`
	IsLoggedIn int            `json:"is_logged_in"`
	Secrets    []types.Secret `json:"secrets"`
}

func (u User) ToResponse() www.User {
	usr := www.User{
		Username:  u.UserName,
		OidcSub:   u.OidcSub,
		AvatarURL: u.AvatarURL,
		Email:     u.Email,
		Active:    u.Active,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Role:      u.Role,
		Secrets:   []www.Secret{},
	}
	for _, ele := range u.Secrets {
		usr.Secrets = append(usr.Secrets, ele.ToResponse())
	}
	return usr
}

func NewUserFromJSON(in string) (User, error) {
	var u User
	if e := json.Unmarshal([]byte(in), &u); e != nil {
		err := errors.Wrapf(e, "Failed to unmarshal user from JSON")
		return u, err
	}
	return u, nil
}

// / SQlite specific methods
func (s Sqlite) InsertUser(ctx context.Context, u types.User) (userID int64, err error) {
	q := `INSERT INTO users (user_name, oidc_sub, avatar_url, email, first_name, last_name, user_active, is_logged_in, created_at, updated_at, role)`
	q += ` VALUES (?, ?,?, ?, ?, ?, ?, ?, ?, ?,?)`
	stmt, err := db.Prepare(q)
	if err != nil {
		return
	}
	res, err := stmt.Exec(u.Username, u.OidcSub, u.AvatarURL, u.Email, u.FirstName, u.LastName, u.Active, u.IsLoggedIn, time.Now(), time.Now(), u.Role)
	if err != nil {
		return
	}
	userID, err = res.LastInsertId()
	if err != nil {
		return
	}
	u.SqlID = userID
	for _, ele := range u.Secrets {
		_, err = s.InsertSecret(ctx, u.SqlID, ele)
		if err != nil {
			return
		}
	}
	return
}

func (s Sqlite) LoginUserWithHash(ctx context.Context, username, pwhash string) (err error) {
	logrus.Debug(">> LoginUser")
	// check login and return token
	u, err := s.GetByUsername(ctx, username)
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

func (s Sqlite) LoginUserScope(ctx context.Context, username, password string, req types.Request) (status int, role string, pl helper.ErrorPayload, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": "sqlite/user.go",
		"func":    "LoginUserScope",
	})
	defer span.End()
	// check login and return token
	u, err := s.GetByUsername(ctx, username)
	if err != nil {
		l.Errorf("user '%s' not found", username)
		pl = helper.NewErrPayload(
			"UNAUTHORIZED",
			"User not found",
			helper.NewDetail("repository", req.Repo, req.Method),
		)
		return http.StatusNotFound, role, pl, nil
	}
	//l.Debugf("Found user '%s' (role:%s) in database", u.UserName, u.Role)
	match, err := u.PasswordMatches(ctx, password)
	if err != nil {
		l.Error(err)
		return http.StatusInternalServerError, role, pl, err
	}
	if !match {
		pl = helper.NewErrPayload(
			"UNAUTHORIZED",
			"wrong credentials",
			helper.NewDetail("repository", req.Repo, req.Method),
		)
		l.Error("wrong credentials")
		return http.StatusUnauthorized, role, pl, nil
	}
	switch req.Method {
	case "PUT", "POST", "DELETE", "PATCH":
		if u.Role != "admin" {
			pl = helper.NewErrPayload(
				"UNAUTHORIZED",
				"wrong privileges",
				helper.NewDetail("repository", req.Repo, req.Method),
			)
			l.Error("wrong privileges for PUT/POST/PATCH/DELETE")
			return http.StatusUnauthorized, role, pl, nil
		}
	}
	return http.StatusOK, u.Role, pl, nil
}

func (s Sqlite) LoginUser(ctx context.Context, username, password string) (err error) {
	l := logrus.WithFields(logrus.Fields{
		"package": pkgNameUser,
		"func":    "LoginUser",
	})
	_ = l
	// check login and return token
	u, err := s.GetByUsername(ctx, username)
	if err != nil {
		return fmt.Errorf("User not found: %s", err.Error())
	}
	match, err := u.PasswordMatches(ctx, password)
	if err != nil {
		return fmt.Errorf("error when matching passwords")
	}
	if !match {
		return fmt.Errorf("password does not match")
	}
	err = s.SetLoggedIn(ctx, u.ID)
	if err != nil {
		return errors.Wrap(err, "error when setting logged in")
	}
	return
}

func (s Sqlite) SetLoggedOut(ctx context.Context, uID int) error {
	q := `UPDATE users SET is_logged_in = 0 WHERE id = ?`
	stmt, err := db.Prepare(q)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(uID)
	return err
}

func (s Sqlite) SetLoggedIn(ctx context.Context, uID int) error {
	q := `UPDATE users SET is_logged_in = 1 WHERE id = ?`
	stmt, err := db.Prepare(q)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(uID)
	return err
}

func (s Sqlite) GetByEmail(ctx context.Context, email string) (*User, error) {
	usr, err := s.User.getByEmail(email)
	if err != nil {
		return nil, err
	}
	if srcts, err := s.GetSecrets(ctx, usr.ID); err != nil {
		return nil, err
	} else {
		usr.Secrets = srcts
	}
	return usr, nil
}

func (s Sqlite) GetOne(ctx context.Context, uid int) (*User, error) {
	usr, err := s.User.getOne(uid)
	if err != nil {
		return nil, err
	}
	if srcts, err := s.GetSecrets(ctx, usr.ID); err != nil {
		return nil, err
	} else {
		usr.Secrets = srcts
	}
	return usr, nil
}

func (s Sqlite) UserNameExists(ctx context.Context, username string) (ok bool, err error) {
	return s.User.userNameExists(username)
}

func (s Sqlite) GetByUsername(ctx context.Context, username string) (*User, error) {
	usr, err := s.User.getByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if srcts, err := s.GetSecrets(ctx, usr.ID); err != nil {
		return nil, err
	} else {
		usr.Secrets = srcts
	}
	return usr, nil
}

func (s Sqlite) CreateWWWUser(ctx context.Context, wwwUser www.User) (int64, error) {
	tUsr := types.UserFromWWWUser(wwwUser, "user")
	return s.InsertUser(ctx, tUsr)
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
		"oidc_sub" TEXT,
		"avatar_url" TEXT,
		"email" TEXT,
		"first_name" TEXT,
		"last_name" TEXT,
		"role" TEXT,
		"password_hash" TEXT,
		"user_active" INTEGER,
		"is_logged_in" INTEGER,
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

func (*User) userNameExists(uname string) (ok bool, err error) {
	rows, err := db.Query("SELECT id FROM users WHERE user_name = ?", uname)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if rows.Next() {
		return true, nil
	}
	return false, nil
}

// GetByEmail returns one user by email
func (u *User) getByUsername(ctx context.Context, username string) (*User, error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUser,
		"func":    "GetByUsername",
	})
	defer span.End()

	//query := `SELECT id, user_name, github_id, avatar_url, email, first_name, last_name, user_active, is_logged_in, created_at, role FROM users WHERE user_name = $1`
	query := fmt.Sprintf(`SELECT id, user_name, oidc_sub, email, first_name, last_name, user_active, is_logged_in, created_at, role FROM users WHERE user_name = '%s'`, username)
	utils.Debug(l, span, "%s", query)
	var user User
	row := db.QueryRowContext(ctx, query)
	err := row.Scan(
		&user.ID,
		&user.UserName,
		&user.OidcSub,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.Active,
		&user.IsLoggedIn,
		&user.CreatedAt,
		&user.Role,
	)
	if err != nil {
		return nil, utils.ErrorHandler(l, span, err, "error when scanning row")
	}
	utils.Debug(l, span, "found user %s with role '%s'", user.UserName, user.Role)
	return &user, nil
}

// GetByEmail returns one user by email
func (u *User) getByEmail(email string) (*User, error) {
	ctx := context.Background()

	//query := `SELECT id, user_name, github_id, avatar_url, email, first_name, last_name, user_active, is_logged_in, created_at, updated_at ,role FROM users WHERE email = $1`
	query := `SELECT id, user_name, email, first_name, last_name, user_active, created_at, updated_at ,role FROM users WHERE email = $1`
	var user User
	row := db.QueryRowContext(ctx, query, email)

	err := row.Scan(
		&user.ID,
		&user.UserName,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.Active,
		//&user.IsLoggedIn,
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
func (u *User) getOne(id int) (*User, error) {
	ctx := context.Background()
	//query := `select id, user_name,github_id, avatar_url, email, first_name, last_name, user_active, is_logged_in, created_at, updated_at, role from users where id = $1`
	query := `select id, user_name, email, first_name, last_name, user_active, created_at, updated_at, role from users  where id = $1`
	var user User
	row := db.QueryRowContext(ctx, query, id)
	err := row.Scan(
		&user.ID,
		&user.UserName,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.Active,
		//&user.IsLoggedIn,
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

	stmt := `update secrets set secret_hash = $1 where user_id = $2`
	_, err = db.ExecContext(ctx, stmt, hashedPassword, u.ID)
	if err != nil {
		return err
	}

	return nil
}

// PasswordMatches uses Go's bcrypt package to compare a user supplied password
// with the hash we have stored for a given user in the database. If the password
// and hash match, we return true; otherwise, we return false.
func (u *User) PasswordMatches(ctx context.Context, plainText string) (bool, error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameUser,
		"func":    "PasswordMatches",
	})
	_ = ctx
	defer span.End()
	granted := false
	utils.Debug(l, span, "checking password against %d secrets", len(u.Secrets))
	for _, secr := range u.Secrets {
		utils.Debug(l, span, "checking secret %s", secr.Key)
		err := bcrypt.CompareHashAndPassword([]byte(secr.SecretHash), []byte(plainText))
		if err != nil {
			switch {
			case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
				// invalid password
				return false, nil
			default:
				utils.Debug(l, span, "error when comparing hash and password: %s", err.Error())
				return false, err
			}
		} else {
			// Once we hit a hash that matches, the user is authorized
			utils.Debug(l, span, "password matches")
			granted = true
			break
		}
	}
	return granted, nil
}

func (u *User) PasswordHashMatches(passwordHash string) (bool, error) {
	for _, secr := range u.Secrets {
		if secr.SecretHash == passwordHash {
			return true, nil
		}
	}
	return false, nil
}
