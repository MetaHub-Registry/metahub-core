package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func mkRuntimeMani(id int64, reg, repo, tag, osName, arch string) Manifest {
	return Manifest{
		ID:       id,
		Upstream: Upstream{Reg: reg, Repo: repo, Tag: tag},
		Platform: Platform{OS: osName, Arch: arch},
	}
}

// TestDedupeLatest_CollapsesSameTagPlatformToHighestID is the core case: a mutable
// runtime tag rebuilt+re-pushed leaves two same-platform manifests; keep the latest.
func TestDedupeLatest_CollapsesSameTagPlatformToHighestID(t *testing.T) {
	in := []Manifest{
		mkRuntimeMani(1285, "registry.gitlab.com", "qnib-pub-containers/ai/llama-cpp", "cuda12-6_sm86-real", "linux", "amd64"),
		mkRuntimeMani(1471, "registry.gitlab.com", "qnib-pub-containers/ai/llama-cpp", "cuda12-6_sm86-real", "linux", "amd64"),
	}
	out := DedupeLatestByUpstreamTagPlatform(in)
	assert.Len(t, out, 1)
	assert.Equal(t, int64(1471), out[0].ID, "keeps the latest (highest-id) push of a re-pushed mutable tag")
}

// TestDedupeLatest_DifferentTagNotCollapsed guards against merging genuinely
// different runtime images that share a platform (rocm vs cuda, both linux/amd64).
func TestDedupeLatest_DifferentTagNotCollapsed(t *testing.T) {
	in := []Manifest{
		mkRuntimeMani(10, "reg", "ai/llama-cpp", "rocm7.2.4_gfx1151", "linux", "amd64"),
		mkRuntimeMani(11, "reg", "ai/llama-cpp", "cuda12-6_sm86-real", "linux", "amd64"),
	}
	out := DedupeLatestByUpstreamTagPlatform(in)
	assert.Len(t, out, 2, "different upstream tags on the same platform must NOT collapse")
}

// TestDedupeLatest_DifferentPlatformNotCollapsed: same tag, different platforms
// (a real multi-arch tag) must keep every platform.
func TestDedupeLatest_DifferentPlatformNotCollapsed(t *testing.T) {
	in := []Manifest{
		mkRuntimeMani(10, "reg", "img", "latest", "linux", "amd64"),
		mkRuntimeMani(11, "reg", "img", "latest", "linux", "arm64"),
	}
	out := DedupeLatestByUpstreamTagPlatform(in)
	assert.Len(t, out, 2, "a multi-arch tag must retain one manifest per platform")
}

// TestDedupeLatest_DifferentRegistryNotCollapsed: same repo:tag under different
// registries are treated as distinct (safe under-collapse, never over-collapse).
func TestDedupeLatest_DifferentRegistryNotCollapsed(t *testing.T) {
	in := []Manifest{
		mkRuntimeMani(10, "regA", "img", "latest", "linux", "amd64"),
		mkRuntimeMani(11, "regB", "img", "latest", "linux", "amd64"),
	}
	out := DedupeLatestByUpstreamTagPlatform(in)
	assert.Len(t, out, 2)
}

// TestDedupeLatest_EmptyUpstreamPassthrough: manifests without a full upstream
// repo+tag cannot be attributed to a mutable tag and must pass through untouched.
func TestDedupeLatest_EmptyUpstreamPassthrough(t *testing.T) {
	in := []Manifest{
		{ID: 1, Platform: Platform{OS: "linux", Arch: "amd64"}},
		{ID: 2, Platform: Platform{OS: "linux", Arch: "amd64"}},
		{ID: 3, Upstream: Upstream{Reg: "r", Repo: "repo"}, Platform: Platform{OS: "linux", Arch: "amd64"}}, // tag empty
	}
	out := DedupeLatestByUpstreamTagPlatform(in)
	assert.Len(t, out, 3, "no full upstream repo+tag => never collapse")
}

// TestDedupeLatest_OrderPreservedAndInPlaceReplace: a later higher-id collision
// replaces the retained manifest in its original slot; unrelated keys keep order.
func TestDedupeLatest_OrderPreservedAndInPlaceReplace(t *testing.T) {
	in := []Manifest{
		mkRuntimeMani(10, "reg", "a", "t", "linux", "amd64"), // key A -> slot 0
		mkRuntimeMani(20, "reg", "b", "t", "linux", "amd64"), // key B -> slot 1
		mkRuntimeMani(15, "reg", "a", "t", "linux", "amd64"), // key A collision, 15 > 10 -> replace slot 0
	}
	out := DedupeLatestByUpstreamTagPlatform(in)
	assert.Len(t, out, 2)
	assert.Equal(t, int64(15), out[0].ID, "higher-id collision replaces in the original slot")
	assert.Equal(t, int64(20), out[1].ID)
}

// TestDedupeLatest_LowerIdCollisionKeepsExisting: an out-of-order lower-id
// duplicate does not displace the already-retained latest.
func TestDedupeLatest_LowerIdCollisionKeepsExisting(t *testing.T) {
	in := []Manifest{
		mkRuntimeMani(20, "reg", "a", "t", "linux", "amd64"),
		mkRuntimeMani(10, "reg", "a", "t", "linux", "amd64"),
	}
	out := DedupeLatestByUpstreamTagPlatform(in)
	assert.Len(t, out, 1)
	assert.Equal(t, int64(20), out[0].ID)
}

// TestDedupeLatest_EmptyAndNil
func TestDedupeLatest_EmptyAndNil(t *testing.T) {
	assert.Empty(t, DedupeLatestByUpstreamTagPlatform(nil))
	assert.Empty(t, DedupeLatestByUpstreamTagPlatform([]Manifest{}))
}

// TestDedupeLatest_RealisticMix mirrors the qnib/llama-server runtime index:
// one rocm base + an old and new cuda base under one mutable tag.
func TestDedupeLatest_RealisticMix(t *testing.T) {
	reg, repo := "registry.gitlab.com", "qnib-pub-containers/ai/llama-cpp"
	in := []Manifest{
		mkRuntimeMani(1267, reg, repo, "rocm7.2.4_gfx1151", "linux", "amd64"),
		mkRuntimeMani(1285, reg, repo, "cuda12-6_sm86-real", "linux", "amd64"), // stale
		mkRuntimeMani(1471, reg, repo, "cuda12-6_sm86-real", "linux", "amd64"), // current (wrapper)
	}
	out := DedupeLatestByUpstreamTagPlatform(in)
	assert.Len(t, out, 2)
	ids := []int64{out[0].ID, out[1].ID}
	assert.Contains(t, ids, int64(1267), "rocm base retained")
	assert.Contains(t, ids, int64(1471), "latest cuda base retained")
	assert.NotContains(t, ids, int64(1285), "stale cuda base dropped")
}
