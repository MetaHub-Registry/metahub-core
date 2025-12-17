package minio

import (
	"context"
	"flag"
	"testing"

	"github.com/sirupsen/logrus"
)

var (
	minioPorts = flag.String("minio-ports", "", "ports for minio test instance")
	min        Minio
	ctx        = context.Background()
)

func TestMain(m *testing.M) {
	logrus.Info("Start TestMain")
	ctx := context.Background()
	minioC, err := SetupMinio(ctx, *minioPorts)
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Warnf("Minio: %v", min)
	bc := map[string]BucketConfig{
		blobBucketKey:      NewBucketConfig(blobBucketKey, minioC.HttpURI, minioC.ConsoleURI),
		manifestsBucketKey: NewBucketConfig(manifestsBucketKey, minioC.HttpURI, minioC.ConsoleURI),
	}
	min, err = NewMiniWithCfg(bc)
	if err != nil {
		logrus.Fatal(err)
	}
	err = min.ConnectClients()
	if err != nil {
		logrus.Fatal(err)
	}
	min.BucketCheck()
	defer minioC.Terminate(ctx)
	code := m.Run()
	switch code {
	case 0:
		logrus.Info("Tests OK")
	default:
		logrus.Infof("Tests FAILED with code %d", code)
	}
}

/*
func SetupTest(bootstrap string) (err error) {
	logrus.Debug("> SetupTest")
	switch bootstrap {
	case "":
	case "cfg:generic":
		err := min.PutConfig(ctx, featureTestGenericCfg, featureTestGenericCfgDigest)
		if err != nil {
			logrus.Fatal(err)
		}

		case "mani:generic":
			err := min.PutConfig(ctx, featureTestGenericCfg, featureTestGenericCfgDigest)
			if err != nil {
				logrus.Fatal(err)
			}
	default:
		return fmt.Errorf("No bootstrap for '%s'", bootstrap)
	}
	return
}
*/
