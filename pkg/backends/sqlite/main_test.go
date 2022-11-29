package sqlite

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

var (
	sq  Sqlite
	ctx = context.Background()
)

func Test_SetupTeardown(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.AddBootstrap("profiles")
	bs.AddBootstrap("platforms")
	bs.AddBootstrap("users")
	bs.AddBootstrap("config:qnib/featuretest:arm64")
	bs.AddBootstrap("upstreams:qnib/featuretest:arm64")
	bs.AddBootstrap("layers:qnib/featuretest:arm64")
	bs.AddBootstrap("manifest:qnib/featuretest:arm64")
	bs.AddBootstrap("manifestlist:qnib/featuretest:arm64")
	bs.AddBootstrap("collection:qnib/featuretest:arm64")
	bs.AddBootstrap("filter:generic")
	br, err := SetupTest(bs)
	_ = br
	assert.NoError(t, err)
	defer TeardownTest(bs)
}

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
	err := sq.EmptyData(bs.SqlEmptyAll)
	if err != nil {
		logrus.Fatal(err)
	}

}

func SetupTest(bs utils.Bootstrap) (br utils.BootstrapResult, err error) {
	logrus.Debug("> SetupTest")
	br = utils.NewBootstrapResult()
	for _, b := range bs.Bootstraps {
		switch b {
		case "profiles":
			err = Setup_Profiles(&sq, &br, "../../../misc/yaml/profiles")
			if err != nil {
				err = errors.Wrap(err, "Setup_Profiles")
				return
			}
		case "platforms":
			err = Setup_Platforms(&sq, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Platforms")
				return
			}
		case "users":
			err = Setup_Users(&sq, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Users")
				return
			}
		case "manifest:qnib/featuretest:arm64":
			err = Setup_Manifest_QNIB_FeatureTest_arm(&sq, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Manifest_QNIB_FeatureTest_arm")
				return
			}
		case "manifest:qnib/featuretest:amd64":
			err = Setup_Manifest_QNIB_FeatureTest_amd(&sq, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Manifest_QNIB_FeatureTest_amd")
				return
			}
		case "config:qnib/featuretest:arm64":
			err = Setup_Config_FTarm(&sq, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Config_FTarm")
				return
			}
		case "config:qnib/featuretest:amd64":
			err = Setup_Config_FT_amd64(&sq, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Config_FT_amd64")
				return
			}

		case "upstreams:qnib/featuretest:arm64":
			err = Setup_Upstream_FTarm(&sq, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Upstream_FTarm")
				return
			}
		case "upstreams:qnib/featuretest:amd64":
			err = Setup_Upstream_FT_amd64(&sq, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Upstream_FT_amd64")
				return
			}
		case "layers:qnib/featuretest:arm64":
			err = Setup_Layers_FTarm(&sq, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Layers_FTarm")
				return
			}
		case "layers:qnib/featuretest:amd64":
			err = Setup_Layers_FT_amd64(&sq, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Layers_FT_amd64")
				return
			}
		case "manifestlist:qnib/featuretest:amd64":
			err = Setup_Manifestlist_FTamd64(&sq, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Manifestlist_FTamd64")
				return
			}

		case "manifestlist:qnib/featuretest:arm64":
			err = Setup_Manifestlist_FTarm(&sq, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Manifestlist_FTarm")
				return
			}
		case "collection:qnib/featuretest:arm64":
			err = Setup_Collection_QNIB_FeatureTest_arm(&sq, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Collection_QNIB_FeatureTest_arm")
				return
			}
		case "collection:qnib/featuretest:amd64":
			err = Setup_Collection_QNIB_FeatureTest_amd64(&sq, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Collection_QNIB_FeatureTest_amd64")
				return
			}
		case "collection:qnib/featuretest:generic":
			err = Setup_Collection_QNIB_FeatureTest_Generic(&sq, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Collection_QNIB_FeatureTest_Generic")
				return
			}
		case "filter:generic":
			err = Setup_Filter_Generic(&sq, &br)
			if err != nil {
				err = errors.Wrap(err, "Setup_Filter_Generic")
				return
			}
		default:
			err = fmt.Errorf("No Setup for '%s'", b)
			return br, err
		}
	}
	return
}
