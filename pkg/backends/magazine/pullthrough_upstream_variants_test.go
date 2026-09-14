package magazine

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/upstream"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// splitImageRef parses "reg/repo:tag" into its three components.
// The registry is the first dot-containing segment; the tag is split by ":".
func splitImageRef(imageRef string) (reg, repo, tag string, err error) {
	// Split tag suffix first.
	repoPart, tag, found := strings.Cut(imageRef, ":")
	if !found || tag == "" {
		return "", "", "", fmt.Errorf("image ref %q has no tag", imageRef)
	}
	// Registry is the first path segment that contains a dot or colon (port).
	idx := strings.Index(repoPart, "/")
	if idx < 0 {
		return "", "", "", fmt.Errorf("image ref %q has no repo path", imageRef)
	}
	reg = repoPart[:idx]
	repo = repoPart[idx+1:]
	if reg == "" || repo == "" {
		return "", "", "", fmt.Errorf("image ref %q: empty reg or repo", imageRef)
	}
	return reg, repo, tag, nil
}

// argsWithoutKey returns a shallow copy of m without the given key.
func argsWithoutKey(m map[string]string, key string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		if k != key {
			out[k] = v
		}
	}
	return out
}

// TestPullThroughAnonUpstreamExplicitConfig verifies that the pull-through
// ingest path works when TEST_REGISTRY is NOT active and the upstream is
// addressed via an explicit RegistryConfig (AllowAnon:true, TLS:false). This
// mirrors the anonymous-pull scenario for harbor.example.com/library/alpine.
//
// The test uses anonRegHttpURI — a second pre-populated test registry started
// in TestMain that carries the same alpine:3.17.3 content as the primary
// test registry but is NOT covered by the TEST_REGISTRY catch-all override.
func TestPullThroughAnonUpstreamExplicitConfig(t *testing.T) {
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.SetEmptyAll()

	ctx := context.Background()
	br, err := mag.SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)

	// Remove the TEST_REGISTRY catch-all so NewRegClient uses the explicit
	// RegistryConfig entry below instead of redirecting to the main test reg.
	prevArgs := mag.Config.Args
	mag.Config.Args = argsWithoutKey(mag.Config.Args, "TEST_REGISTRY")
	defer func() { mag.Config.Args = prevArgs }()

	prevRegs := mag.Config.Registries
	mag.Config.Registries = []RegistryConfig{{
		Name:        anonRegHttpURI,
		AllowAnon:   true,
		TLS:         false,
		PullThrough: PullThroughConfig{Enabled: true},
	}}
	defer func() { mag.Config.Registries = prevRegs }()

	const repo = "alpine"
	const tag = "3.17.3"
	req := types.Request{
		Host:    anonRegHttpURI,
		Repo:    repo,
		Tag:     tag,
		Profile: "generic",
	}
	blob := types.BlobMeta{
		MediaType: MediaTypeManifest,
		Digest:    shab5a,
	}
	require.NoError(t, mag.UpstreamBlobInsertManifest(ctx, req, blob),
		"anon pull-through via explicit RegistryConfig must succeed")

	upQ := fmt.Sprintf(
		`MATCH (u:Upstream) WHERE u.repo = "%s" AND u.tag = "%s" RETURN count(u) AS n`,
		repo, tag)
	res, found, err := mag.Database.QueryOne(upQ)
	require.NoError(t, err)
	require.True(t, found)
	n, err := res.GetInt64("n")
	require.NoError(t, err)
	require.GreaterOrEqual(t, n, int64(1), "Upstream node must exist after anon pull-through ingest")
}

// TestPullThroughHarborPrivate is an integration test against a real private
// Harbor repository. It is skipped unless all three env vars are set:
//
//	HARBOR_PRIVATE_USER      — Harbor username (robot account or user)
//	HARBOR_PRIVATE_PASSWORD  — Harbor password or robot secret
//	HARBOR_PRIVATE_IMAGE     — full image ref, e.g. harbor.example.com/private/alpine:3.23
//
// The test verifies that pull-through succeeds when credentials are wired via
// RegistryConfig.User / RegistryConfig.Password.
func TestPullThroughHarborPrivate(t *testing.T) {
	harborUser := os.Getenv("HARBOR_PRIVATE_USER")
	harborPass := os.Getenv("HARBOR_PRIVATE_PASSWORD")
	harborImage := os.Getenv("HARBOR_PRIVATE_IMAGE")
	if harborUser == "" || harborPass == "" || harborImage == "" {
		t.Skip("HARBOR_PRIVATE_USER, HARBOR_PRIVATE_PASSWORD, HARBOR_PRIVATE_IMAGE not set")
	}

	reg, repo, tag, err := splitImageRef(harborImage)
	require.NoError(t, err, "HARBOR_PRIVATE_IMAGE must be in reg/repo:tag format")

	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.SetEmptyAll()

	ctx := context.Background()
	br, err := mag.SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)

	prevArgs := mag.Config.Args
	mag.Config.Args = argsWithoutKey(mag.Config.Args, "TEST_REGISTRY")
	defer func() { mag.Config.Args = prevArgs }()

	prevRegs := mag.Config.Registries
	mag.Config.Registries = []RegistryConfig{{
		Name:      reg,
		AllowAnon: false,
		TLS:       true,
		User:      harborUser,
		Password:  harborPass,
		PullThrough: PullThroughConfig{Enabled: true},
	}}
	defer func() { mag.Config.Registries = prevRegs }()

	// HEAD the manifest to obtain its content-addressed digest.
	upt := upstream.UpstreamTriple{Reg: reg, Repo: repo, Tag: tag}
	_, blobMeta, err := mag.GetManifestByUpstreamTripleByTag(ctx, upt)
	require.NoError(t, err, "manifest HEAD from Harbor private repo must succeed (credentials injected)")

	req := types.Request{Host: reg, Repo: repo, Tag: tag, Profile: "generic"}
	require.NoError(t, mag.UpstreamBlobInsertManifest(ctx, req, blobMeta),
		"pull-through ingest from Harbor private repo must succeed")

	upQ := fmt.Sprintf(
		`MATCH (u:Upstream) WHERE u.repo = "%s" AND u.tag = "%s" RETURN count(u) AS n`,
		repo, tag)
	res, found, err := mag.Database.QueryOne(upQ)
	require.NoError(t, err)
	require.True(t, found)
	n, err := res.GetInt64("n")
	require.NoError(t, err)
	require.GreaterOrEqual(t, n, int64(1), "Upstream node must exist after Harbor private pull-through")
}

// TestPullThroughGitLabPrivate is an integration test against a real private
// repository on registry.gitlab.com. It is skipped unless all three env vars
// are set:
//
//	GITLAB_USER           — GitLab username
//	GITLAB_TOKEN          — personal access token with read_registry scope
//	GITLAB_PRIVATE_IMAGE  — full image ref, e.g. registry.gitlab.com/mygroup/myrepo:tag
//
// The test verifies that pull-through succeeds when credentials are wired via
// RegistryConfig.User / RegistryConfig.Password (token passed as password).
func TestPullThroughGitLabPrivate(t *testing.T) {
	gitlabUser := os.Getenv("GITLAB_USER")
	gitlabToken := os.Getenv("GITLAB_TOKEN")
	gitlabImage := os.Getenv("GITLAB_PRIVATE_IMAGE")
	if gitlabUser == "" || gitlabToken == "" || gitlabImage == "" {
		t.Skip("GITLAB_USER, GITLAB_TOKEN, GITLAB_PRIVATE_IMAGE not set")
	}

	reg, repo, tag, err := splitImageRef(gitlabImage)
	require.NoError(t, err, "GITLAB_PRIVATE_IMAGE must be in reg/repo:tag format")

	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:amd64")
	bs.AddBootstrap("rules:rules/cpu/noop.yaml")
	bs.AddBootstrap("rules:rules/os/linux.yaml")
	bs.AddBootstrap("profile:profiles/generic.yaml")
	bs.SetEmptyAll()

	ctx := context.Background()
	br, err := mag.SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)

	prevArgs := mag.Config.Args
	mag.Config.Args = argsWithoutKey(mag.Config.Args, "TEST_REGISTRY")
	defer func() { mag.Config.Args = prevArgs }()

	prevRegs := mag.Config.Registries
	mag.Config.Registries = []RegistryConfig{{
		Name:      reg,
		AllowAnon: false,
		TLS:       true,
		User:      gitlabUser,
		Password:  gitlabToken,
		PullThrough: PullThroughConfig{Enabled: true},
	}}
	defer func() { mag.Config.Registries = prevRegs }()

	// HEAD the manifest to obtain its content-addressed digest.
	upt := upstream.UpstreamTriple{Reg: reg, Repo: repo, Tag: tag}
	_, blobMeta, err := mag.GetManifestByUpstreamTripleByTag(ctx, upt)
	require.NoError(t, err, "manifest HEAD from registry.gitlab.com must succeed (token as password)")

	req := types.Request{Host: reg, Repo: repo, Tag: tag, Profile: "generic"}
	require.NoError(t, mag.UpstreamBlobInsertManifest(ctx, req, blobMeta),
		"pull-through ingest from registry.gitlab.com must succeed")

	upQ := fmt.Sprintf(
		`MATCH (u:Upstream) WHERE u.repo = "%s" AND u.tag = "%s" RETURN count(u) AS n`,
		repo, tag)
	res, found, err := mag.Database.QueryOne(upQ)
	require.NoError(t, err)
	require.True(t, found)
	n, err := res.GetInt64("n")
	require.NoError(t, err)
	require.GreaterOrEqual(t, n, int64(1), "Upstream node must exist after GitLab pull-through")
}
