package upstream

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
)

var regHttpURI string

func TestMain(m *testing.M) {
	logrus.Info("Start TestMain")
	/// Registry
	regC, err := SetupRegistry(ctx)
	if err != nil {
		logrus.Fatal(err)
	}
	defer regC.Terminate(ctx)
	regHttpURI = regC.HttpURI
	///
	code := m.Run()
	os.Exit(code)
	logrus.Info("End TestMain")
}
