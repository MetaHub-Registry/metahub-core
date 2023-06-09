package magazine

import (
	"context"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

const (
	pkgNameRegcliTest = "pkg/backends/magazine/regcli_test.go"
)

func TestNewRegclient(t *testing.T) {
	magLocal := Magazine{
		Config: MagConfig{
			Args: map[string]string{
				"TEST_REGISTRY": "http://localhost:5000",
			},
		},
	}
	rc, regOut, err := magLocal.NewRegClient(ctx, "registry:5050")
	if err != nil {
		t.Fatal(err)
	}
	if rc == nil {
		t.Fatal("rc is nil")
	}
	assert.Equal(t, "http://localhost:5000", regOut)
	// Actual test
}

func TestGetManifestByUpstreamTripleByDigest(t *testing.T) {
	// Using the local test registry
	ctx := context.Background()
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:arm64")
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := mag.SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	TeardownTest(bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs)
	// Test
	up := upstream.UpstreamTriple{
		Reg:    "registry:5050",
		Repo:   "qnib-pub-containers/qnib/plain-featuretest",
		Tag:    "20220715-arm64-generic",
		Digest: digest.Digest("sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5"),
	}
	if _, blob, found, err := mag.getManifestByUpstreamTripleByDigest(ctx, up); err != nil {
		t.Fatal(err)
	} else if !found {
		t.Fatal("Did not find manifest")
	} else {
		t.Logf("Found manifest: %v", blob)
	}
	if _, blob, err := mag.GetManifestByUpstreamTripleByDigest(ctx, up); err != nil {
		t.Fatal(err)
	} else {
		t.Logf("Found manifest: %v", blob)
	}
	delete(mag.Config.Args, "cache-registry")
	if _, blob, err := mag.GetManifestByUpstreamTripleByDigest(ctx, up); err != nil {
		t.Fatal(err)
	} else {
		t.Logf("Found manifest: %v", blob)
	}
}

func TestGetManifestByUpstreamTripleByTag(t *testing.T) {
	// Using the local test registry
	debug := 0
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:arm64")
	bs.SetEmptyAll()
	if debug > 0 {
		bs.PostTestSleep = debug
	}
	br, err := mag.SetupTest(ctx, bs)
	for _, l := range br.ToLines() {
		if debug > 0 {
			t.Log(l)
		}
	}
	TeardownTest(bs)
	if err != nil {
		t.Fatal(err)
	}
	defer TeardownTest(bs)
	// Test
	up := upstream.UpstreamTriple{
		Reg:    "registry:5050",
		Repo:   "qnib-pub-containers/qnib/plain-featuretest",
		Tag:    "20220715-arm64-generic",
		Digest: digest.Digest("sha256:ecb2919cfe22d74996ed677453f4e6f3bfd2806a7b591697a2fbea9ebee95db5"),
	}
	if _, blob, err := mag.GetManifestByUpstreamTripleByTag(ctx, up); err != nil {
		t.Fatal(err)
	} else {
		t.Logf("Found manifest: %v", blob)
	}
	delete(mag.Config.Args, "cache-registry")
	if _, blob, err := mag.GetManifestByUpstreamTripleByTag(ctx, up); err != nil {
		t.Fatal(err)
	} else {
		t.Logf("Found manifest: %v", blob)
	}
}
