package neo

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestMain(m *testing.M) {
	logrus.Info("Start TestMain")
	code := m.Run()
	os.Exit(code)
	logrus.Info("End TestMain")
}
