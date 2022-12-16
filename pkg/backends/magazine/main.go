package magazine

import (
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/metahub-registry/metahub-core/pkg/backends/minio"
	"github.com/metahub-registry/metahub-core/pkg/backends/neo"
	"github.com/metahub-registry/metahub-core/pkg/backends/sqlite"
)

type Magazine struct {
	Minio          minio.Minio
	Database       Database
	UserManagement sqlite.Sqlite
	//SQlite sqlite.Sqlite
	//Neo    neo.Neo
}

func NewMagazine(cmd *cobra.Command, tables string) (m Magazine, err error) {
	cfgFile, _ := cmd.Flags().GetString("config")
	min, err := minio.NewMinio(cfgFile)
	if err != nil {
		err = errors.Wrapf(err, "failed to create minio backend")
		logrus.Fatal(err)
	}
	err = min.ConnectClients()
	if err != nil {
		err = errors.Wrapf(err, "failed to connect clients")
		logrus.Fatal(err)
	}

	err = min.BucketCheck()
	if err != nil {
		err = errors.Wrapf(err, "failed to execute BucketCheck")
		logrus.Fatal(err)
	}
	m = Magazine{
		Minio: min,
	}
	// user
	userSqlFile, _ := cmd.Flags().GetString("user-db")
	m.UserManagement, err = sqlite.New(userSqlFile, "users,tokens")
	if err != nil {
		err = errors.Wrapf(err, "failed to create sqlite backend for users")
		logrus.Fatal(err)
	}
	// Neo4j
	neo4j, err := neo.New(cfgFile)
	if err != nil {
		err = errors.Wrapf(err, "failed to create neo4j backend")
		logrus.Fatal(err)
	}
	m.Database = neo4j
	err = m.Database.Setup()
	if err != nil {
		err = errors.Wrapf(err, "failed to setup neo4j backend")
		logrus.Fatal(err)

	}
	return
}
