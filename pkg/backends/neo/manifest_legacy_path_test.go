package neo

// Coverage for the ManifestGetByCollectionTag legacy path and the pure record
// helpers introduced with the collection-scoped attribution work:
//
//   - manifestColTagLegacy / processLegacyManifestRecord (wrapper-shaped
//     fallback for manifests pushed before the slot-centred model)
//   - applyContentSize (int64/int/float64 switch arms)
//   - hydrateOCIManifestRecord error branches (malformed records)

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

// fakeNeoResult wraps a single *neo4j.Record as a neo4j.Result so the pure
// record helpers can be exercised without a database round-trip.
type fakeNeoResult struct {
	rec *neo4j.Record
}

func (f *fakeNeoResult) Keys() ([]string, error)              { return f.rec.Keys, nil }
func (f *fakeNeoResult) Next() bool                           { return false }
func (f *fakeNeoResult) NextRecord(**neo4j.Record) bool       { return false }
func (f *fakeNeoResult) Err() error                           { return nil }
func (f *fakeNeoResult) Record() *neo4j.Record                { return f.rec }
func (f *fakeNeoResult) Collect() ([]*neo4j.Record, error)    { return nil, nil }
func (f *fakeNeoResult) Single() (*neo4j.Record, error)       { return f.rec, nil }
func (f *fakeNeoResult) Consume() (neo4j.ResultSummary, error) { return nil, nil }

func TestApplyContentSize(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		vals []interface{}
		want int64
	}{
		{name: "missing_contentSize", keys: []string{"other"}, vals: []interface{}{"x"}, want: 0},
		{name: "nil_contentSize", keys: []string{"contentSize"}, vals: []interface{}{nil}, want: 0},
		{name: "int64", keys: []string{"contentSize"}, vals: []interface{}{int64(42)}, want: 42},
		{name: "int", keys: []string{"contentSize"}, vals: []interface{}{int(43)}, want: 43},
		{name: "float64", keys: []string{"contentSize"}, vals: []interface{}{float64(44.0)}, want: 44},
		{name: "unsupported_type", keys: []string{"contentSize"}, vals: []interface{}{"45"}, want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := &fakeNeoResult{rec: &neo4j.Record{Keys: tc.keys, Values: tc.vals}}
			manifest := &types.Manifest{}
			applyContentSize(res, manifest)
			assert.Equal(t, tc.want, manifest.ContentSize)
		})
	}
}

// TestHydrateOCIManifestRecord_ErrorPaths walks the record-decoder error
// branches by feeding progressively valid synthetic records.
func TestHydrateOCIManifestRecord_ErrorPaths(t *testing.T) {
	platformKeys := []string{"platformID", "platformArch", "platformOS", "platformVariant"}
	platformVals := []interface{}{int64(1), "amd64", "linux", ""}

	tests := []struct {
		name string
		keys []string
		vals []interface{}
	}{
		{
			name: "missing_platform",
			keys: []string{"foo"}, vals: []interface{}{"bar"},
		},
		{
			name: "missing_config",
			keys: append(platformKeys, "unrelated"), vals: append(platformVals, "x"),
		},
		{
			name: "missing_upstream",
			keys: append(platformKeys, "configID", "configDigest", "configSize"),
			vals: append(platformVals, int64(2), "sha256:cfg", int64(10)),
		},
		{
			name: "missing_manifestData",
			keys: append(platformKeys, "configID", "configDigest", "configSize",
				"upstreamID", "upstreamReg", "upstreamRepo", "upstreamTag", "upstreamDigest"),
			vals: append(platformVals, int64(2), "sha256:cfg", int64(10),
				int64(3), "reg", "repo", "tag", "sha256:up"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l, ctx, span := utils.InitLogging(context.Background(), tracer, map[string]any{
				"package": "neo/manifest_legacy_path_test.go",
				"func":    "TestHydrateOCIManifestRecord_ErrorPaths",
			})
			defer span.End()
			res := &fakeNeoResult{rec: &neo4j.Record{Keys: tc.keys, Values: tc.vals}}
			_, err := db.hydrateOCIManifestRecord(ctx, l, span, res, "test", map[int64][]string{})
			assert.Error(t, err, "a record missing mandatory keys must not hydrate")
		})
	}
}

// legacyColTagFixture wires the deprecated wrapper-shaped graph behind
// manifestColTagLegacy: Repo/Tag -UPSTREAM-> ManifestWrapper -r-> Manifest
// with config blob, platform and upstream node. Extra edges tune the
// profile-matching outcome.
func legacyColTagFixture(t *testing.T, repo string, withProfile, withFailureAnno bool) (colID, colTagID int64) {
	t.Helper()
	colID, colTagID = colTagFixture(t, repo, "v1")

	mdDigest := "sha256:" + strings.Repeat("b"+fmt.Sprint(len(repo)), 32)
	prfLabel := "legacy-" + repo
	extra := ""
	if withProfile {
		extra += fmt.Sprintf(`
		CREATE (prf:Profile {name: %q, tag: %q, rule: "legacy", label: %q})
		CREATE (prf)-[:USES]->(rule:Rule {op: "eq", type: "anno", value: "legacy"})
		CREATE (rule)-[:TRUE]->(p)`, prfLabel, prfLabel, prfLabel)
	}
	if withFailureAnno {
		extra += `
		CREATE (a:Annotation {key: "legacy.anno", value: "bad"})
		CREATE (md)-[:HAS_ANNOTATION]->(a)
		CREATE (a)<-[:FALSE]-(frule:Rule {op: "eq", type: "anno", value: "legacy.anno=bad"})
		CREATE (frule)<-[:USES]-(prf)`
	}
	q := fmt.Sprintf(`
		MATCH (r:Repo {repo: %q})-[:HAS_TAG]->(t:Tag {name: "v1"})
		CREATE (m:ManifestWrapper {digest: "sha256:%s", mediaType: "application/vnd.oci.image.index.v1+json", size: 900, label: %q})
		CREATE (md:Manifest {digest: %q, mediaType: "application/vnd.oci.image.manifest.v1+json", size: 500, label: %q})
		CREATE (cfg:Blob {digest: "sha256:%s", mediaType: "application/vnd.oci.image.config.v1+json", size: 77, type: "config", label: "legacycfg"})
		CREATE (p:Platform {os: "linux", arch: "amd64", variant: "", label: %q})
		CREATE (u:Upstream {reg: "reg.example", repo: "legacy/stream", tag: "v1", digest: %q})
		CREATE (r)-[:UPSTREAM]->(m)
		CREATE (t)-[:UPSTREAM]->(m)
		CREATE (m)-[:REFERENCES]->(md)
		CREATE (m)-[:ORIGINAL]->(u)
		CREATE (md)-[:USES]->(cfg)
		CREATE (md)-[:HAS_PLATFORM]->(p)`+extra,
		repo,
		strings.Repeat("a"+fmt.Sprint(len(repo)), 32),
		repo+"wrap",
		mdDigest,
		repo+"md",
		strings.Repeat("c"+fmt.Sprint(len(repo)), 32),
		repo+":linux/amd64",
		mdDigest)
	_, err := db.Execute(q)
	require.NoError(t, err)
	return colID, colTagID
}

func TestManifestGetByCollectionTag_LegacyPath(t *testing.T) {
	ctx := context.Background()
	defer slotTestSetup(ctx, t)()

	t.Run("matching_profile_appends_manifest", func(t *testing.T) {
		colID, colTagID := legacyColTagFixture(t, "qnib/legacy-match", true, false)
		manis, err := db.ManifestGetByCollectionTag(ctx, colID, colTagID)
		require.NoError(t, err)
		require.Len(t, manis, 1)
		m := manis[0]
		assert.Equal(t, "sha256:"+strings.Repeat("b17", 32), m.Digest)
		assert.Equal(t, "legacy-qnib/legacy-match", m.Profile, "legacy path attributes the manifest to the matched profile")
		assert.Equal(t, "legacy-qnib/legacy-match", m.Upstream.Profile)
		assert.Equal(t, "REFERENCES", m.Type)
		assert.Equal(t, "reg.example", m.Upstream.Reg)
		assert.Equal(t, "linux/amd64", m.Platform.GetLabel())
		assert.Equal(t, int64(77), m.Config.Size)
	})

	t.Run("failing_annotation_filters_manifest", func(t *testing.T) {
		colID, colTagID := legacyColTagFixture(t, "qnib/legacy-fail", true, true)
		manis, err := db.ManifestGetByCollectionTag(ctx, colID, colTagID)
		require.NoError(t, err)
		assert.Empty(t, manis, "a FAILURE-annotated manifest must be filtered out of the legacy path")
	})

	t.Run("no_profile_yields_no_manifest", func(t *testing.T) {
		colID, colTagID := legacyColTagFixture(t, "qnib/legacy-noprof", false, false)
		manis, err := db.ManifestGetByCollectionTag(ctx, colID, colTagID)
		require.NoError(t, err)
		assert.Empty(t, manis)
	})
}
