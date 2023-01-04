package magazine

import (
	"fmt"
	"os"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/metahub-registry/metahub-core/pkg/backends/minio"
	"github.com/metahub-registry/metahub-core/pkg/backends/neo"
	"github.com/metahub-registry/metahub-core/pkg/backends/sqlite"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

type Magazine struct {
	Minio          minio.Minio
	Database       Database
	UserManagement sqlite.Sqlite
	Config         MagConfig
}

func NewMagazine(cmd *cobra.Command, tables string) (m Magazine, err error) {
	cfgFile, _ := cmd.Flags().GetString("config")
	// viper
	viper.SetConfigType("yaml")
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".cobra-example" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".metahub-admin")
		viper.SetConfigType("yaml")
	}
	err = viper.ReadInConfig()
	if err != nil {
		logrus.Fatalf("Error reading config file, %s", err)
	}

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
	fr := viper.GetString("frontend.redirect")
	au := viper.GetString("auth.url")
	cfg := MagConfig{
		RedirectDoc: fr,
		AuthURL:     au,
	}
	m = Magazine{
		Minio:  min,
		Config: cfg,
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

func (mag Magazine) SetupTest(bs utils.Bootstrap) (br utils.BootstrapResult, err error) {
	logrus.Info("> SetupTest")
	//return
	br = utils.NewBootstrapResult()
	for _, b := range bs.Bootstraps {
		switch b {
		// Users/Profile/Platforms
		case "users":
			err = sqlite.Setup_Users(&mag.UserManagement, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Users")
				return
			}
		default:
			err = mag.Database.BootstrapTestWithBR(b, bs, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Users in Database")
				return
			}
			err = mag.Minio.BootstrapTestWithBR(b, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Users in Minio")
				return
			}
		}
	}
	return
}
