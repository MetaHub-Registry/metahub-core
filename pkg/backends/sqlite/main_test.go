package sqlite

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
)

var sq Sqlite

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

func Test_Collection_GetAll(t *testing.T) {
	// Need to insert all of the other tables as well to fire up the big query
}
