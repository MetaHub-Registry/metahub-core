# Context: metalib

> This file helps AI agents understand the repo quickly.
> Updated after each session that modifies this repo.

## What This Repo Is

The shared Go library for MetaHub. Provides the `magazine.Magazine`
interface, neo4j backend implementation, type definitions, and all
Cypher queries. Both metahub-api and metahub-admin depend on this.

## Architecture

```
pkg/
  backends/
    magazine/              Magazine interface + orchestration
      main.go              Magazine struct and interface definition
      collection.go        Collection/repo operations
      profile.go           Profile operations
      manifest.go          Manifest CRUD
      compute.go           Compute operations (manifestlists, etc.)
      filter.go            Filter logic
      platform.go          Platform matching
    neo/                   Neo4j backend (the main backend)
      collection_listing.go  Collection listing Cypher queries
      collection_tag.go      Collection-by-tag queries
      profiles.go            Profile queries
      rule.go                Rule evaluation queries
      rule_invariants.go     D5 invariant helper
      manifest.go            Manifest CRUD queries
      descriptor.go          Descriptor/referrers queries
      index.go               Index operations
      slot.go                Slot operations
      repo.go                Repo node queries
      filter.go              Filter queries
    minio/                 MinIO blob storage backend
    sqlite/                SQLite backend (legacy, mostly dead)
  types/                   Shared type definitions
    profiles.go            Profile type
    rules.go               Rule type
    collection_listing.go  CollectionListing type
    collection_filter_pair.go  Filter pair for UI dropdowns
    manifest.go            Manifest type (with ContentSize)
    index.go               Index type
    slot.go                Slot type
    repo.go                Repo type
    platform.go            Platform type
  yaml/config/             YAML config parsing (profiles, rules, collections)
  version/                 Version string (bumped by bump2version)
  utils/                   Shared utilities (expr, regex, etc.)
```

## Key Patterns

### Three Graph Patterns

The slot-centred migration left three coexisting patterns. Any query
walking from Repo to Manifest must handle all three:

1. **Legacy**: `(:Repo)-[:UPSTREAM]->(:ManifestWrapper)-[:WRAPS]->(:Manifest)`
2. **Slot-centred**: `(:Repo)-[:HAS_TAG]->(:Tag)-[:POINTS_TO]->(:Index)-[:REFERENCES]->(:Manifest)`
3. **Direct UPSTREAM**: `(:Repo)-[:UPSTREAM]->(:Manifest)`

Pattern 2 is the target. Patterns 1 and 3 are backward compatibility.
See `CollectionListingGetByRepoLike` for the canonical three-branch query.

### Magazine Interface

The `magazine.Magazine` interface is the contract between metahub-api
handlers and the backend. All data access goes through this interface.
The neo4j backend (`pkg/backends/neo/`) is the primary implementation.

### Rule Evaluation

Rules use `expr` expressions evaluated against manifest properties.
`AssertRuleShortcutInvariants` (D5) validates rule-shortcut consistency.

## Testing

```bash
./test.sh 2>&1 > test.log    # full suite with coverage
```

Neo4j tests need a running neo4j instance (see docker-compose or
use the dev environment).

## Recent Changes

- Referrers `:Index` label fix in `descriptor.go`
- Three graph pattern fallback in collection listing/tag queries
- `content_size` (config + layer blobs) added to manifest queries
- Dev versioning with `-devN` suffix
- SonarQube version auto-detection

## Gotchas

- `ManifestWrapper` label still exists for backward compat — don't delete
- `collect(DISTINCT blob)` dedup is critical in content_size queries
- 19 legacy functions still use ManifestWrapper/UPSTREAM patterns
- `compute manifestlists` doesn't produce `:Index` for CE-parsed data
- `artifactType` not populated in referrers response descriptors

## 2026-04-18 (ui-and-manipulation session)

### What changed
- **Extended manipulation schema** (`extend-manipulation-schema`):
  - New types: `ManipuConfig`, `ManipuCommand` (op=`replace|prepend|append`
    + `value []string`), `ManipuVariant`.
  - `Manipulation` now carries `Default` and `Variants` in addition to
    the legacy `ManipuLayers` slice.
  - YAML parser at `pkg/yaml/config/manipulation.go` detects legacy vs
    new form and rejects mixed use. Legacy form still parses with a
    deprecation warning and is internally rewritten to variants keyed
    by each layer's rule.
  - Neo4j backend: `(:Manipulation)-[:HAS_DEFAULT|HAS_VARIANT {rule}]->(:ManipuConfig)`
    with `HAS_ENTRYPOINT`/`HAS_CMD` → `(:ManipuCommand)` and
    `HAS_LAYER {order}` → `(:ManipuLayer)`. Env + Volumes + command
    Value are stored as JSON strings so they round-trip through the
    existing `types.Result` scalar helper.
  - New `Database.ManipulationsWithConfigByProfileID` returns the
    fully-hydrated tree for downstream UI exposure.
- **Fixtures**: `misc/yaml/manipulations/layer/example-full/1.0.0.yaml`
  (every new field) and `example-minimal/1.0.0.yaml` (variants-only).
- **CE compatibility tests**: `pkg/yaml/config/manipulation_ce_test.go`
  loads the migrated community-edition YAMLs to verify the parser keeps
  working on shipped data.
- **Drive-by**: `pkg/backends/neo/repo_coverage_test.go` tolerates
  Neo4j node ID `0` (legitimate for the first node in a freshly emptied
  graph).

### Key files
- `pkg/yaml/config/manipulation.go` — extended parser + validator
- `pkg/yaml/config/manipuconfig.go`, `manipucommand.go`,
  `manipuconversion.go` — new schema + config↔types converters
- `pkg/types/manipuconfig.go`, `manipucommand.go`, `manipuvariant.go`
- `pkg/backends/neo/manipuconfig.go` — ManipuConfig / ManipuCommand
  CRUD helpers
- `pkg/backends/neo/manipulation.go` — extended
  `ManipulationCreate`, `ManipulationFromResult`,
  `ManipulationFromConfigManipulation`; added
  `ManipulationsWithConfigByProfileID`
- `pkg/backends/magazine/database.go` — new interface method
- Tests: `manipulation_extended_test.go`,
  `manipuconfig_test.go`, `manipuconfig_extended_test.go`,
  `manipuconfig_test.go` (types), `manipuconversion_test.go`,
  `manipulation_ce_test.go`

### Gotchas
- `ProfileGetInsert` early-returns when `p.Rule == ""`, which means a
  profile with no rule string will never link to its manipulations.
  Test fixtures must include a parseable rule.
- Neo4j map properties are awkward to read back through the current
  driver + `types.Result`; env and volumes are stored as JSON strings
  instead of native map/list types.
- `manipuConfigLayers` calls `ManipuLayerFromResult` which in turn
  requires an attached Manifest; the helper augments the returned
  layer's `Image.Upstream` via `UpstreamGetByManipuLayerID` so the UI
  DTO can render an image URL.
- **Variant layers skip the legacy Rule linkage**. `ManipuLayer`
  uniqueness is `(label, rule)` and the old `ManipuLayerFromUpstream`
  pipeline linked each layer to a `:Rule` node by label. Variant
  layers carry their rule on the `HAS_VARIANT` edge instead, so they
  use the new `manipuLayerCreateForConfig` helper (bare ManipuLayer
  + UPSTREAM + Manifest link, no Rule node).
- **Apply-time flatten matters**. `ApplyManipulationsToManifests`
  used to iterate only `mp.ManipuLayers` which is empty under the new
  schema. `flattenManipuLayers` now walks legacy flat layers +
  Default.Layers + each variant's layers, copying the variant's rule
  string onto each variant layer for platform matching. Without this
  flatten the compose-stack glow pull-through returns the base image
  unmodified.

## 2026-04-17 Continuation

### What changed
- **RegistryConfig Name/URL split**: New `Name`, `URL` fields with `EffectiveName()`/`EffectiveURL()` helpers. Legacy `Hostname` field retained for backward compat. `normalize()` bridges Name<->Hostname. `ValidateRegistries()` fail-fast check at config load time.
- **Magazine.GetBlobRedirectURL**: TLS-aware blob redirect URL builder. Consults `RegistryConfig.TLS` to emit `http://` or `https://`. Matches on both `EffectiveURL` and `EffectiveName`. `Upstream.GetBlobUrl` deprecated but retained.
- **DiscoverUpstream prefix stripping**: When `targetReg` is set, strips the slug prefix from the repo path for upstream HEAD calls. `upstreamInsertGeneric` stores the stripped repo on Upstream nodes.
- **ManifestGetManipulatedByOriginalAndLabel platform hydration**: Added `OPTIONAL MATCH (md)-[:HAS_PLATFORM]->(p:Platform)` to the dedup query so returned manipulated manifests carry correct platform fields. Without this, Docker refused to pull ("no matching manifest for linux/arm64/v8").

### Key files
- `pkg/backends/magazine/magconfig.go` — Name/URL/EffectiveName/EffectiveURL/ValidateRegistries
- `pkg/backends/magazine/magconfig_test.go` — 30+ tests covering all config scenarios + GetBlobRedirectURL
- `pkg/backends/magazine/regcli.go` — GetBlobRedirectURL, NewRegClient TLS lookup updated
- `pkg/backends/magazine/upstream.go` — DiscoverUpstream prefix stripping, AllowsUnauthorizedPull dual-match
- `pkg/backends/neo/manifest.go` — ManifestGetManipulatedByOriginalAndLabel platform fix
- `pkg/types/upstream.go` — GetBlobUrl deprecated with doc comment

### Gotchas
- `ResolveUpstreamRegistry` returns `EffectiveURL()` (not the slug Name) — callers like `resolveUpstreamTarget` get the network endpoint
- Upstream nodes store the stripped repo (e.g. `os/alpine317`) not the full prefix alias (`local-reg/os/alpine317`) — this is by design for correct GetBlobUrl/GetManifestByUpstreamByDigest construction

## 2026-06-08 deprecate-manifest-wrapper (milestone: Phases 1–3)

### What changed
- **Dead/test-only `ManifestWrapper*` surface removed** (no behaviour change):
  `HeadManistNotFoundForProfile` + `ManifestWrapperGetByRepoTag`; the OS-annotation
  family (`ManifestWrapperGetOS`/`UpsertOS`/`UpsertOSVersion`/`UpsertOSID`/
  `GetAllSystemAnnos`) from impl + `magazine.Database` interface;
  `ProfileFromManifestWrapperIDViaAnnotations`/`ViaPlatform`. OS annotations
  already live on the content-leaf (`ManifestAddAnnotations` /
  `GetAnnotationsByManifestID`).
- **New Slot/leaf reshape entrypoint** `Magazine.ComputeManifestForLeaf(ctx,
  types.Manifest)` (`compute_manifest.go`): reshapes a pushed Docker v2 manifest
  into OCI v1 per profile **without a caller-supplied wrapper**. It resolves the
  wrapper identity internally from the content-leaf
  (`ManifestWrapperGetIDByManifestID` → `ManifestWrapperGetByID`) and delegates to
  the existing `ComputeManifest` — byte-identical output. This severs the
  metahub-api ↔ wrapper coupling.
- **Stopped creating `:ManifestWrapper` nodes** in live paths:
  `CollectionGetCreateUpstreamTriple` (`repo.go`) no longer calls
  `ManifestWrapperGetInsertWithDesc`; `ManifestDelete` (`manifest.go`) wrapper
  cleanup is a no-op. (The descriptor-ingest path was already production-dead —
  its only caller `upstreamBlobInsertManifestDesc` is commented out.)
- **Slot-centred fixture wiring** (`setupSlotForLeaf` in
  `test_setup_manifest_content.go`): MERGEs a `:Slot` for `(repo,tag,profile)` +
  `:HAS_PUSHED` to the content-leaf. Additive — wrapper-node creation in fixtures
  is retained until the read-Cypher migration.

### Key files
- `pkg/backends/magazine/compute_manifest.go` — `ComputeManifestForLeaf`
- `pkg/backends/magazine/database.go` — interface: +`ManifestWrapperGetIDByManifestID`, −OS family
- `pkg/backends/neo/manifest_wrapper_deprecated.go` — dead surface removed (still present; full deletion is Phase 5)
- `pkg/backends/neo/{profiles,collection,repo,manifest}.go` — dead methods removed / wrapper creation stopped
- `pkg/backends/neo/test_setup_manifest_content.go` — `setupSlotForLeaf`
- `pkg/backends/{magazine/compute_manifest_for_leaf_test.go, neo/test_setup_slot_test.go, neo/upstream_triple_slot_test.go}` — new tests

### Deferred (separate OpenSpec changes)
- **`migrate-wrapper-read-cypher`** (proposed): migrate remaining `:ManifestWrapper`
  read Cypher to Slot/leaf walks; re-point fixtures off wrapper attachment; drop
  `ComputeManifestForLeaf`'s internal wrapper resolution. Carries a LIVE/DEAD
  reachability audit. **Must land atomically** — a partial attempt this session
  cascaded 8+ test failures and was reverted.
- **Phase 5** (not yet a change): delete `manifest_wrapper_deprecated.go` +
  `types.ManifestWrapper` + interface entries; one-time `DETACH DELETE` migration.

### Gotchas
- **Fixture key trap**: `manifest:<label>#id` = legacy **wrapper** node id;
  content-leaf `:Manifest` = `manifestdata:<label>#id`. Linking a Repo/Tag to the
  wrapper id breaks listing once `LinkCollection*ToManifest` is tightened to
  `(m:Manifest)`.
- **neo4j id 0**: first node in an emptied graph gets internal id 0.
- **`ComputeManifest` rejects type `ORIGINAL`** (only PARSED/UPLOADED_BY/
  MANIFEST_DERIVED_FROM reshape) — relevant when choosing multi-arch fixtures.
- sonar (`sonar.example.com`) and VictoriaLogs unreachable off-network.

---

## Phase 4 — `migrate-wrapper-read-cypher` (COMPLETE)

Removed all LIVE `:ManifestWrapper` graph reads; the Slot/content-leaf model is
now the only path exercised by production code and fixtures. `./test.sh` is
green (EXIT 0). Sonar skipped (off-network).

### What changed
- **Fixtures (`test_setup*.go`)**: stopped creating `:ManifestWrapper` nodes.
  `Bootstrap_ManifestWrapper` now just aliases `manifest:<label>#id` to the
  content-leaf id. `Setup_Manifest`/`Setup_OrasManifest` set both
  `manifestdata:<label>#id` and the `manifest:<label>#id` alias and no longer
  call `LinkManifestWrapperToManifest`. `Setup_Collection_FT_*` link Repo/Tag to
  the content-leaf via the new tolerant `linkCollectionToLeaves` helper
  (skips/defer-links when `collection:` is bootstrapped before `manifestdata:`).
  Scenario setup reordered to create the content-leaf before the collection.
- **Dual-path queries tightened to `(m:Manifest)`**: `repo.go LinkCollectionToManifest`,
  `collection_tag.go LinkCollectionTagToManifest`, `manifest.go manifestQueryByRepoTag`
  (wrapper fallback dropped), `collection_listing.go` (wrapper EXISTS branch dropped),
  `manifest.go ManifestV1GetInsert` (`maniSiID>0` MERGE block removed; param now ignored).
- **6 LIVE wrapper-only queries rekeyed onto the content-leaf id** (walk via
  `:ORIGINAL`/`:HAS_PUSHED`/`:POINTS_TO`/`:REFERENCES`/`:HAS_PLATFORM`):
  `ManifestGetByProfileMediaType`, `ManifestGetRelIDByMediaType`,
  `ManifestGetAllByManifestID`, `ManifestGetByManifestList` (manifest.go),
  `RulesGetByManifestID` (rule.go), `LayerGetUpstream` (layer.go).
- **Magazine**: `ComputeManifestForLeaf` builds a leaf-keyed `types.ManifestWrapper`
  shim (`mani.ID == leaf.ID`) instead of resolving a wrapper node; defaults empty
  `Type` to `ORIGINAL` and skips when the leaf is absent. `CreateOCIv1` now reshapes
  `ORIGINAL` (grouped with `PARSED`). Synthetic wrappers in `upstream.go` keyed on
  the content-leaf id. `ManifestWrapperGetIDByManifestID` removed from the
  `magazine.Database` interface.
- **Tests**: wrapper-era tests converted to the leaf model (manifest_test.go CRUD/
  Links/Rules, manifest_wrapper_test.go GetByLabel→GetByDigest, index_test.go,
  manifest_coverage5_test.go, magazine compute/oci/manifestlist tests now drive the
  leaf shim / `ComputeManifestForLeaf`). `TestFixturesAreSlotCentred` asserts zero
  `:ManifestWrapper` nodes after setup.

### Known residue (intentional — Phase 5)
The final `grep ":ManifestWrapper"` over prod (non-test, non-deprecated-file)
still shows query strings in **DEAD** functions (no live caller; all references
are inside `manifest_wrapper_deprecated.go` or interface decls):
`ManifestGetParsedNodeRelByMediaType`, `ManifestGetIDByMediaType`,
`ManifestGetAll`, `ManifestGetAllByManifestIDWithProfile`,
`ManifestExistsForTagProfileContent`, `CollectionExists`,
`GetCollectionFilterPairs*`, `ProfileManifestType`, `UpstreamGetIDByManifestID`,
`UpstreamCleanupByID`, `CollectionTagGetIDByManifestID`,
`GetAnnotationsByManifestWrapperID`, `annotations.go`. Per the change scope these
are left for Phase 5 deletion, not rewritten.

### Gotcha discovered
- The computed OCI manifest has **no graph edge back to the original content-leaf**
  (the wrapper `CREATED` edge that linked them is gone). Leaf-keyed "already
  present?" checks therefore rely on the Slot/Index sibling walk or the
  profile linkage (`CREATED_BY`/`UPLOADED_BY`), not a direct leaf→OCI edge. Tests
  that verified "a new OCI manifest exists for profile X" were switched to walk
  `(:Manifest)-[:CREATED_BY]->(:Profile)` directly.
