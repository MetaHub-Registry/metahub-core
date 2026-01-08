package minio

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/tracing"
)

var (
	min     Minio
	ctx     = context.Background()
	svcName = "mino-test"
)

func TestMain(m *testing.M) {
	logrus.Info("Start TestMain")
	ctx := context.Background()
	minioP := os.Getenv("MINIO_PORTS")
	minioC, err := SetupMinio(ctx, minioP)
	if err != nil {
		logrus.Fatal(err)
	}
	//jeager
	jeagerThriftURI := ""
	//jeagerHttpURI := ""
	jeagerBase := os.Getenv("JAEGER_BASE")
	if jeagerBase == "" {
		jeagerC, err := tracing.SetupJeager(ctx)
		if err != nil {
			logrus.Fatal(err)
		}
		defer jeagerC.Terminate(ctx)
		//jeagerHttpURI = jeagerC.HttpURI
		jeagerThriftURI = jeagerC.ThriftURI
	} else {
		//jeagerHttpURI = fmt.Sprintf("http://%s:16686", jeagerBase)
		jeagerThriftURI = fmt.Sprintf("%s:14268", jeagerBase)
	}
	trEnd := ""
	if strings.HasPrefix(jeagerThriftURI, "http://") {
		trEnd = fmt.Sprintf("%s/api/traces", jeagerThriftURI)
	} else {
		trEnd = fmt.Sprintf("http://%s/api/traces", jeagerThriftURI)
	}
	tr, err := tracing.InitTracer(svcName, trEnd)
	if err != nil {
		logrus.Fatalf("Could not initialize jaeger tracer: %s", err.Error())
	}
	trCfg := tracing.TraceCfg{
		Enabled:  true,
		Endpoint: jeagerThriftURI,
		Tracer:   tr,
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
	min.Config = MinioConfig{
		Tracing: trCfg,
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
