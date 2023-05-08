package sqlite

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

var (
	sq Sqlite
	//ctx = context.Background()
)

func TestMain(m *testing.M) {
	f, err := os.CreateTemp("", "sqlite-test")
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Infof("Start TestMain: %s", f.Name())
	defer os.Remove(f.Name())
	sq, err = New(f.Name(), "")
	if err != nil {
		logrus.Fatal(err)
	}
	code := m.Run()
	os.Exit(code)
	logrus.Info("End TestMain")
}

func TeardownTest(bs utils.Bootstrap) {
	if bs.CopySqlite != "" {
		logrus.Infof("Copy SQlite DB to '%s'", bs.CopySqlite)
		sq.Close()
		utils.CopyFile(sq.MhFile, bs.CopySqlite)
		err := sq.ReOpenDB()
		if err != nil {
			logrus.Fatal(err)
		}
	}
	err := sq.EmptyData(bs.EmptyAll)
	if err != nil {
		logrus.Fatal(err)
	}

}

func SetupTest(bs utils.Bootstrap) (br utils.BootstrapResult, err error) {
	logrus.Debug("> SetupTest")
	return sq.BootstrapTest(bs)
}
