package neo

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/metahub-registry/metahub-core/pkg/tracing"
	"github.com/metahub-registry/metahub-core/pkg/utils"
	"github.com/sirupsen/logrus"
)

var (
	db      Neo
	ctx     context.Context
	svcName = "neo-test"
)

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		// The package fixture starts Neo4j and Jaeger containers. The public
		// fast lane runs with -short and no container runtime; the integration
		// lane runs the full suite. See openspec/oss-public-ci.
		os.Exit(0)
	}
	logrus.Info("Start TestMain")
	ctx := context.Background()
	neoP := os.Getenv("NEO4J_PORTS")
	neoC, err := SetupNeo4j(ctx, neoP)
	if err != nil {
		logrus.Fatal(err)
	}
	// Container cleanup is done explicitly before os.Exit at the end
	// (defers don't run after os.Exit)
	jeagerC, err := tracing.SetupJeager(ctx)
	if err != nil {
		logrus.Fatal(err)
	}
	trEnd := fmt.Sprintf("http://%s/api/traces", jeagerC.ThriftURI)
	logrus.Infof("Tracing endpoint '%s': %s", svcName, trEnd)
	tr, err := tracing.InitTracer(svcName, trEnd)
	if err != nil {
		logrus.Fatalf("Could not initialize jaeger tracer: %s", err.Error())
	}
	trCfg := tracing.TraceCfg{
		Enabled:  true,
		Endpoint: jeagerC.ThriftURI,
		Tracer:   tr,
	}
	ncfg := NeoConfig{
		DbName:  neoDb,
		User:    neoUser,
		Passwd:  neoPasswd,
		URL:     neoC.BoltURI,
		HTTP:    neoC.HttpURI,
		Tracing: trCfg,
		Args: map[string]string{
			"TEST_JAEGER_HTTP":   jeagerC.HttpURI,
			"TEST_JAEGER_THRIFT": jeagerC.ThriftURI,
		},
	}

	db = Neo{
		Enabled:   true,
		neoConfig: ncfg,
	}
	err = db.Connect()
	if err != nil {
		logrus.Fatal(err)
	}
	err = db.Setup(ctx)
	if err != nil {
		logrus.Fatal(err)
	}
	code := m.Run()
	// Shut down the tracer provider to flush spans and stop goroutines.
	// This prevents the otel batch span processor goroutine leak that
	// causes exit code 1 even when all tests pass.
	if err := tracing.Shutdown(ctx); err != nil {
		logrus.Warnf("tracer shutdown: %v", err)
	}
	// Explicitly terminate containers before os.Exit (defers won't run)
	neoC.Terminate(ctx)
	jeagerC.Terminate(ctx)
	os.Exit(code)
}

func SetupTest(ctx context.Context, bs utils.Bootstrap) (br utils.BootstrapResult, err error) {
	logrus.Debug("> SetupTest")
	return db.BootstrapTest(ctx, bs)
}

func TestNeoTest(t *testing.T) {
	//ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	br, err := SetupTest(ctx, bs)
	_ = br
	if err != nil {
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)
	// Actual Tests
	_, err = db.Query("MATCH (n) RETURN n")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Execute("CREATE (n:Test {name: 'test'})")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Query("MATCH (n:Test) RETURN n.name AS name")
	if err != nil {
		t.Fatal(err)
	}
}
