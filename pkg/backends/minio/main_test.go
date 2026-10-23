package minio

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/metahub-registry/metahub-core/pkg/tracing"
	"github.com/sirupsen/logrus"
)

var (
	min     Minio
	ctx     = context.Background()
	svcName = "mino-test"
)

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		// The package fixture starts MinIO and, unless JAEGER_BASE is set, a
		// Jaeger container. The public fast lane runs with -short and no
		// container runtime; the integration lane runs the full suite.
		os.Exit(0)
	}
	logrus.Info("Start TestMain")
	ctx := context.Background()
	minioP := os.Getenv("MINIO_PORTS")
	minioC, err := SetupMinio(ctx, minioP)
	if err != nil {
		logrus.Fatal(err)
	}
	// Container cleanup is done explicitly before os.Exit at the end
	// (defers don't run after os.Exit)
	var jeagerC *tracing.JeagerContainer
	jeagerThriftURI := ""
	jeagerBase := os.Getenv("JAEGER_BASE")
	if jeagerBase == "" {
		jeagerC, err = tracing.SetupJeager(ctx)
		if err != nil {
			logrus.Fatal(err)
		}
		jeagerThriftURI = jeagerC.ThriftURI
	} else {
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
	code := m.Run()
	// Shut down the tracer provider to flush spans and stop goroutines.
	if err := tracing.Shutdown(ctx); err != nil {
		logrus.Warnf("tracer shutdown: %v", err)
	}
	// Explicitly terminate containers before os.Exit (defers won't run)
	minioC.Terminate(ctx)
	if jeagerC != nil {
		jeagerC.Terminate(ctx)
	}
	os.Exit(code)
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
