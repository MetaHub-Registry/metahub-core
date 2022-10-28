package magazine

import (
	"context"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/backends/minio"
	"github.com/metahub-registry/metahub-core/pkg/backends/sqlite"
)

var mag Magazine

func TestMain(m *testing.M) {
	logrus.Info("Start TestMain")
	ctx := context.Background()
	/// Minio
	minioC, err := minio.SetupMinio(ctx)
	if err != nil {
		logrus.Fatal(err)
	}
	configsBucketKey := minio.GetConfigsBucketKey()
	layersBucketKey := minio.GetLayersBucketKey()
	manifestsBucketKey := minio.GetManifestsBucketKey()
	bc := map[string]minio.BucketConfig{
		configsBucketKey:   minio.NewBucketConfig(configsBucketKey, minioC.HttpURI),
		layersBucketKey:    minio.NewBucketConfig(layersBucketKey, minioC.HttpURI),
		manifestsBucketKey: minio.NewBucketConfig(manifestsBucketKey, minioC.HttpURI),
	}
	min, err := minio.NewMiniWithCfg(bc)
	if err != nil {
		logrus.Fatal(err)
	}
	err = min.ConnectClients()
	if err != nil {
		logrus.Fatal(err)
	}
	min.BucketCheck()
	/// SQLite
	f, err := os.CreateTemp("", "sqlite-test")
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Infof("Start TestMain: %s", f.Name())
	defer os.Remove(f.Name())
	sq, err := sqlite.New(f.Name(), "")
	if err != nil {
		logrus.Fatal(err)
	}
	mag = Magazine{
		Minio:  min,
		SQlite: sq,
	}
	code := m.Run()
	os.Exit(code)
	defer minioC.Terminate(ctx)
	logrus.Info("End TestMain")
}
