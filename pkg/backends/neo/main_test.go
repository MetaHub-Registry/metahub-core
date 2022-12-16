package neo

import (
	"context"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

var db Neo

func TestMain(m *testing.M) {
	logrus.Info("Start TestMain")
	ctx := context.Background()
	neoC, err := SetupNeo4j(ctx)
	if err != nil {
		logrus.Fatal(err)
	}
	defer neoC.Terminate(ctx)
	ncfg := NeoConfig{
		DbName: neoDb,
		User:   neoUser,
		Passwd: neoPasswd,
		URL:    neoC.BoltURI,
		HTTP:   neoC.HttpURI,
	}
	db = Neo{
		Enabled:   true,
		neoConfig: ncfg,
	}
	err = db.Connect()
	if err != nil {
		logrus.Fatal(err)
	}
	err = db.Setup()
	if err != nil {
		logrus.Fatal(err)
	}
	code := m.Run()
	os.Exit(code)
	logrus.Info("End TestMain")
}

func SetupTest(bs utils.Bootstrap) (br utils.BootstrapResult, err error) {
	logrus.Debug("> SetupTest")
	return db.BootstrapTest(bs)
}

func Test_NeoTest(t *testing.T) {
	//ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer db.TeardownTest(bs)
	// Actual Tests
	_, err = db.Query("MATCH (n) RETURN n")
	assert.NoError(t, err)
	_, err = db.Execute("CREATE (n:Test {name: 'test'})")
	assert.NoError(t, err)
	_, err = db.Query("MATCH (n:Test) RETURN n.name AS name")
	assert.NoError(t, err)
}
