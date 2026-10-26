package sqlite

import (
	"context"
	"fmt"
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgNameSqlSecrets   = "sqlite/secrets.go"
	errFailDeleteSecret = "Failed to delete secret"
)

type Secret struct {
	/*
		  ID         int       `json:"id"`
			UserID     int       `json:"user_id"`
			Key        string    `json:"secret_key"`
			IsCopied   bool      `json:"is_copied"`
			CreatedAt  time.Time `json:"created_at"`
			SecretHash string    `json:"secret_hash"`
	*/
}

// / User specific methods
func (scrt Secret) CreateTables() (err error) {
	tableName := "secrets"
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
    "user_id" INTEGER NOT NULL,
	"name" TEXT,
	"key" TEXT,
    "secret_hash" TEXT,
    "is_copied" BOOLEAN,
	"created_at" TIMESTAMP,
	"expiry" TIMESTAMP,
	FOREIGN KEY (user_id)
		REFERENCES users (id)
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

func (s Sqlite) InsertSecret(ctx context.Context, uID int64, srct types.Secret) (scrtID int64, err error) {
	q := `INSERT INTO secrets (user_id, name, secret_hash, created_at)`
	q += ` VALUES (?, ?, ?, ?)`
	stmt, err := db.Prepare(q)
	if err != nil {
		return
	}
	hash := utils.GenPasswordHash(srct.Key)
	res, err := stmt.Exec(uID, srct.Name, hash, srct.CreatedAt)
	if err != nil {
		return
	}
	scrtID, err = res.LastInsertId()
	if err != nil {
		return
	}
	srct.SqlID = scrtID
	return
}

func (s Sqlite) GetSecrets(ctx context.Context, uID int) (srcts []types.Secret, err error) {
	rows, err := db.Query("SELECT id, name, secret_hash, created_at FROM secrets WHERE user_id = ?", uID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var scrts []types.Secret
	for rows.Next() {
		var scrt types.Secret
		if err := rows.Scan(&scrt.SqlID, &scrt.Name, &scrt.SecretHash, &scrt.CreatedAt); err != nil {
			return scrts, err
		}
		scrts = append(scrts, scrt)
	}
	if err = rows.Err(); err != nil {
		return scrts, err
	}
	return scrts, nil
}

func (s Sqlite) DelSecret(ctx context.Context, uID int, sID string) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package":  pkgNameSqlSecrets,
		"func":     "DelSecretByName",
		"secretID": sID,
		"uID":      uID,
	})
	defer span.End()
	_ = ctx
	q := `DELETE FROM secrets WHERE id = ? AND user_id = ?`
	utils.Debug(l, span, "%s", q)
	stmt, err := db.Prepare(q)
	if err != nil {
		return
	}
	sInt, err := strconv.Atoi(sID)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, "Failed to convert ID to int")
		return
	}
	utils.Debug(l, span, "id=%d // user_id=%d", sInt, uID)
	res, e := stmt.Exec(sInt, uID)
	if e != nil {
		err = utils.ErrorHandler(l, span, e, errFailDeleteSecret)
		return
	}
	row, e := res.RowsAffected()
	if e != nil {
		err = utils.ErrorHandler(l, span, e, errFailDeleteSecret)
		return
	}

	if row == 0 {
		err = utils.NewError(l, span, "Failed to delete secret, 0 rows affected")

		return
	}
	utils.Debug(l, span, "Deleted secret %d", row)
	return
}

func (s Sqlite) DelSecretByName(ctx context.Context, uID int, sName string) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": pkgNameSqlSecrets,
		"func":    "DelSecretByName",
		"name":    sName,
		"uID":     uID,
	})
	defer span.End()
	_ = ctx
	q := `DELETE FROM secrets WHERE name = ? AND user_id = ?`
	stmt, err := db.Prepare(q)
	if err != nil {
		return
	}
	res, err := stmt.Exec(sName, uID)
	if err != nil {
		err = utils.ErrorHandler(l, span, err, errFailDeleteSecret)
		return
	}
	row, e := res.RowsAffected()
	if e != nil {
		err = utils.ErrorHandler(l, span, e, errFailDeleteSecret)
		return
	}
	if row == 0 {
		err = utils.NewError(l, span, "Failed to delete secret, 0 rows affected")
		return
	}
	utils.Debug(l, span, "Deleted secret '%s' for user '%d'", sName, uID)
	return
}
