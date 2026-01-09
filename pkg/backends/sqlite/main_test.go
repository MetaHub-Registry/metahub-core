package sqlite

import (
	"context"
	"os"
	"testing"
	"time"

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
	switch code {
	case 0:
		logrus.Info("Tests OK")
	default:
		logrus.Infof("Tests FAILED with code %d", code)
	}

}

func TeardownTest(bs utils.Bootstrap, br utils.BootstrapResult) {
	if bs.PostTestSleep > 0 {
		// Start SQLiteBroser
		sqbC, err := SetupSQBrowser(context.Background(), sq.MhFile, "")
		if err != nil {
			logrus.Fatal(err)
		}
		defer func() {
			if err := sqbC.Terminate(context.Background()); err != nil {
				logrus.Fatal(err)
			}
		}()
		logrus.Infof("SQLiteBrowser HTTP URI: %s", sqbC.HttpURI)

		logrus.Infof("Sleeping %d seconds for inspection...", bs.PostTestSleep)
		time.Sleep(time.Duration(bs.PostTestSleep) * time.Second)
	}
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

func SetupTest(ctx context.Context, bs utils.Bootstrap) (br utils.BootstrapResult, err error) {
	logrus.Debug("> SetupTest")

	return sq.BootstrapTest(ctx, bs)
}
