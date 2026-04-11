package sqlite

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/metahub-registry/metahub-core/pkg/yaml/config"
)

func Setup_Users(ctx context.Context, sqObj *Sqlite, br *utils.BootstrapResult) (err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"func": "Setup_Users",
	})
	_ = l
	defer span.End()
	usrs := []config.User{
		{Username: "test", Email: "test@test.de", FirstName: "Max", LastName: "Mustermann", Password: "test", Role: "user"},
		{Username: "admin", Email: "admin@test.de", FirstName: "Admin", LastName: "Mustermann", Password: "admin", Role: "admin"},
	}
	for _, user := range usrs {
		scrt := types.Secret{
			Key:       user.Password,
			Name:      "test0",
			CreatedAt: time.Now(),
			IsCopied:  false,
		}

		u := types.User{
			Username:  user.Username,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Email:     user.Email,
			Role:      user.Role,
			Secrets: []types.Secret{
				scrt,
			},
		}
		if uID, err := sqObj.InsertUser(ctx, u); err != nil {
			return err
		} else {
			br.Add(fmt.Sprintf("user:%s#id", u.Username), fmt.Sprintf("%d", uID))
		}
		bearerToken := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", u.Username, user.Password)))
		br.Add(fmt.Sprintf("user:%s#bearer", u.Username), bearerToken)
		br.Add(fmt.Sprintf("user:%s#password", u.Username), user.Password)
	}
	return
}

func (s Sqlite) BootstrapTest(ctx context.Context, bs utils.Bootstrap) (br utils.BootstrapResult, err error) {
	br = utils.NewBootstrapResult()
	for _, b := range bs.Bootstraps {
		switch b {
		case "users":
			err = Setup_Users(ctx, &s, &br)
			if err != nil {
				logrus.Errorf("Setup_Users: %v", err)
				return
			}
		default:
			logrus.Warnf("sqlite.BootstrapTest: unsupported bootstrap key '%s' (registry data is now in neo4j)", b)
		}
	}
	return
}
