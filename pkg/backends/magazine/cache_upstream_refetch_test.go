package magazine

import (
	"context"
	"testing"

	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// TestCacheUpstreamFoundButNotCached guards the regression where
// cacheManifestByUpstream returned a zero-value Manifest (empty Config.Digest)
// when the Upstream node existed in the graph (found=true) but its
// manifest/config bytes were never cached in MinIO — the exact state
// parse-collections leaves for docker.io/gitlab.com-hosted collection images,
// which record Upstream nodes without caching their blobs.
//
// Before the fix the upstream re-fetch was gated on `if !found`, so the
// found-but-uncached case fell through and returned an empty Manifest; the
// downstream ManifestUpsertBackends -> EnsureBlobExists("") then failed with
// MinIO's "Object name cannot be empty". That aborted pull-time model injection
// for non-pusher profiles (handleIndex served the un-manipulated original), so a
// profile-scoped pull was byte-identical to the generic pull.
func TestCacheUpstreamFoundButNotCached(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.AddBootstrap("platforms:arm64")
	bs.SetEmptyAll()
	br, err := mag.SetupTest(ctx, bs)
	if err != nil {
		TeardownTest(bs, br)
		t.Fatal(err)
	}
	defer TeardownTest(bs, br)

	up := types.UpstreamGlow150ARM64
	// Seed ONLY the Upstream node (found=true) without caching any bytes in
	// MinIO — reproducing the parse-collections state.
	if _, e := mag.NeoInsertUpstream(ctx, up); e != nil {
		t.Fatalf("seed upstream node: %v", e)
	}
	// Precondition: the manifest is genuinely NOT cached in MinIO, so we are
	// exercising the found-but-not-cached path.
	if _, isCached, _ := mag.Minio.GetCachedManifest(ctx, up.Digest); isCached {
		t.Fatal("precondition failed: manifest must not be cached in MinIO")
	}

	// With the fix, CacheUpstream must re-fetch from upstream and succeed
	// (previously: "failed to EnsureBlobExists: ... Object name cannot be empty").
	if e := mag.CacheUpstream(ctx, up); e != nil {
		t.Fatalf("CacheUpstream (found-but-not-cached) must re-fetch, got: %v", e)
	}

	// The re-fetched manifest must resolve with a non-empty config digest.
	req := types.Request{Host: up.Reg, Repo: up.Repo, Tag: up.Tag}
	md, found, err := mag.Database.ManifestGetTypesByRequest(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("manifest data not found after re-fetch")
	}
	if md.Config.Digest == "" {
		t.Fatal("regression: re-fetched manifest carries an empty config digest")
	}
}
