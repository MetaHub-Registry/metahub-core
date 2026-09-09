# Context: metalib

> This file helps AI agents understand the repo quickly.

## Session: waldur-outfitter-improvements — manifest-existence fixes (2026-09-08)

Two correctness fixes shipped on branch `137-waldur-outfitter-improvements`.

### fix-digest-manifest-existence (WP #41, commits `33f59e7` + `a448aaf`)

**Problem:** `HEAD /v2/<repo>/manifests/<digest>` returned 500 when the manifest
existed in the graph but had no `(p:Profile)-[:REF]->` edge (e.g. pushed but
compute sweep not yet run, or an upstream-only manifest). `ManifestFromResult`
treated "unlinked" as an error; the handler mapped any error to 500.

**Fix:** `ManifestFromResult` now returns `ok=false, nil` (no error) when the
manifest has no profile link — i.e. "found but unlinked". The digest-branch of
`ManifestExistsContent` hydrates **without** a profile (content-addressable).
Tag-branch unlinked → `found=false`. metahub-api's Path Z
(`resolveMissingManifest` → `handleManifest` → `ManifestGetByDigest`) already
served without profile checks; no handler change was needed.

**Tests:** `manifest_digest_exists_test.go` (`TestManifestExists_UnlinkedDigestIsNotAnError`).

### align-manifest-existence-queries (WP #42, commit `d4564b2`)

**Problem:** `ManifestListExists` had a Slot-POINTS_TO fallback (added `19a0327`)
but `IndexGetByRequest` used only the strict REF-edge join → HEAD 200 / GET 404
for a freshly-pushed (Slot-wired, REF-less) index.

**Fix:**
- `IndexGetByRequest` gains `indexViaSlot` helper + Slot fallback (mirrors
  `ManifestListExists`).
- `ManifestListExists` tag branch gains `AND NOT (:Slot)-[:HAS_HISTORY]->(ml)`
  (mirrors `IndexGetByRequest`'s pre-existing exclusion so both paths agree on
  what is hidden).

**Tests:** 4-test regression suite in `pkg/backends/neo/`:
- `TestHeadGetAgree_SlotWiredRefLessIndex` — Slot-wired, REF-less → both paths 200
- `TestHeadGetAgree_HasHistoryHiddenOnBothPaths` — HAS_HISTORY → both paths hidden

**Gotchas:**
- `indexViaSlot` must not return an index that is in `HAS_HISTORY` — the
  Slot's CURRENT `POINTS_TO` target is never in `HAS_HISTORY` (by construction),
  so the fallback is safe without an extra exclusion clause.
- Sonar gate deferred to GoCD `metalib_MR137_dev` on session push (sonar.example.com
  off-network at commit time); same pattern as WPs #40/#41.

## Session: waldur-outfitter-improvements — SonarQube gate closure (2026-09-07)

Branch `137-waldur-outfitter-improvements` now passes the full quality gate
(GoCD `metalib_MR137_dev` run 16, commit `c884210`):
`new_violations 0`, `new_duplicated_lines_density 0.0%` (≤3), `new_coverage
88.3%` (≥80). What moved the needle:

**Commits:**
- `2097f8d` S107 fix — `manifestColTagOCI` took 10 params; the five
  query-shaping strings are now bundled in `colTagOCIQuery{pathLabel, relType,
  matchQ, withVars, upstreamQ}`; `manifestColTagSlot`/`manifestColTagUpstream`
  construct the spec.
- `be00427` slot.go dedupe — extracted `slotsFromQuery(l, span, fn, q)`
  (SlotGetAllForRepoTag/SlotGetAllForRepo twins) and
  `latestPushTimestampsFromQuery(l, span, fn, q)`
  (SlotLatestPushTimestamps/RepoTagLatestPushTimestamps twins). That removed
  ALL 38 new duplicated lines (the only new-code duplication in the branch).
- `c884210` coverage — new `pkg/backends/neo/manifest_legacy_path_test.go`:
  legacy wrapper-path fixture (Repo/Tag-UPSTREAM→ManifestWrapper-REFERENCES→
  Manifest) driving `manifestColTagLegacy`/`processLegacyManifestRecord`
  (0%→82.5%) in three flavors (matched profile / FALSE-annotation filtered /
  no profile); `applyContentSize` unit table via a `fakeNeoResult` wrapping
  `neo4j.Record{Keys, Values}` (57%→100%); `hydrateOCIManifestRecord` error
  branches with progressively-valid synthetic records (80%→96%).

**Gotchas learned (do not rediscover):**
- Rule match relations are `[:TRUE]` / `[:FALSE]` (`types.RelationSuccess` /
  `RelationFailure`) — NOT `SUCCESS`/`FAILURE`. A fixture with `[:SUCCESS]`
  silently matches zero profiles.
- `Platform.label` and `Profile.label` are uniqueness-constrained; tests
  sharing one graph (per-TEST `slotTestSetup`, not per-subtest) must use
  per-repo labels AND per-repo Platform nodes — a shared MERGEd Platform lets
  one subtest's `Rule-[:TRUE]->Platform` match another subtest's manifest
  through `HAS_PLATFORM`.
- `Platform.GetLabel()` derives from os/arch/variant, not the `label` property.
- `neo4j.Result` is trivially fakeable in tests (8 methods); `neo4j.Record`
  has exported `Keys`/`Values` — no DB needed for record-decoder coverage.
- Sonar `new_*` per-file measures live under `measures[].period.value`; the
  Sonar token `squ_…` (user-provided) HAS `api/duplications/show` +
  `api/measures/component` privileges (the GoCD `sqa_…` token did not).
- Duplication hunt shortcut: `api/duplications/show` per file gives exact
  block pairs; cross-check with per-file `new_duplicated_lines` to see which
  blocks are in the new-code period.

**Deferred / downstream:** metahub-api dev image (`:17`) embeds metalib at
docker build; the runtime fixes (stale index serving, profile attribution)
warrant a rebuild to `:18` + flux bump — see session CONTEXT.md "Downstream".

## model-centric-pull (session isc-session-v2, 2026-07-07) — branch `132-isc-session-v2`

Complete metalib foundation for model-centric pull (`docker run …/models/<org>/<name>:<quant>`).
All slices additive / tested / pushed.

**Commits & key files:**
- `cc1cf8a` Hardware schema — `pkg/types/hardware.go` (`IsModelServing`/`VRAMBytes`/`FieldMap`); added to `config.Profile` (`pkg/yaml/config/profiles.go`) + `types.Profile`.
- `da2fa57` Neo persistence — `pkg/backends/neo/profile_hardware.go` (pure `hardwareCreateProps`/`hardwareFromResult`) wired into `CreateProfile` + `returnStatementProfile` + `ProfileFromResult`.
- `73e90cb` `ResolveModelServe(quants, label, capacityBytes, namedQuant, headroom)` — capacity from `hardware.gpu.vram_gb`; **dropped `ParseAggregatedProfile`/`vramTierRe`**. Gate in `magazine/manipulation.go` = `prf.Hardware.IsModelServing()`.
- `859a195` override engine — `pkg/utils/expr_override.go` (`RuleSpecificity`, `EvalHardwareRule`), `pkg/types/override.go` (`ResolveOverride`).
- `ca9f864` `models/` loader — `pkg/yaml/config/models.go` (`FindModels`/`Model`/`Validate`).
- `1194295` (#8) quant threading — `pkg/types/request_context.go` (`WithRequestedQuant`), read in `tryApplyModelManipulation`.

**Gotchas:** lep equality is a single `=` (NOT `==`, which fails to parse). `ProfileGetInsert` writes Hardware on CREATE only. `returnStatementProfile` projects `hw_*` → all its queries must alias the Profile node as `prf`.

**Next (#8):** metahub-api rewrites `models/<name>:<tag>` → the model's runtime collection + `ctx=WithRequestedQuant(tag)`; the existing manipulation engine injects model+config. See DevDiary `2026-07-07-1438-model-centric-pull-implementation`.
> Updated after each session that modifies this repo.

## Session: waldur-outfitter-integration — gate-model-serving-on-model-centric (2026-08-24)

### What changed

`applyManipulationsToCandidate` (`pkg/backends/magazine/manipulation.go`)
gated the model-serving stack on the **pull type**, not just the profile:

```go
if prf.Hardware.IsModelServing() && types.RequestedModelFromContext(ctx) != "" {
    return mag.applyModelServingStack(...)
}
// static path skips model/ and config/ manipulations when RequestedModel is empty
```

Before, `RequestedModel` empty (collection pull under a model-serving profile)
still fired `applyModelServingStack` → injected model+config onto a runtime tag,
making `qnib/llama-server:latest` byte-identical to `models/qnib/minicpm5:q8-0`.

Now:
- **Collection pulls** (`qnib/llama-server:latest`, `qnib/ftest:latest`) under a
  model-serving profile are served **un-injected** — the static path runs and
  skips `model/`/`config/` manipulations (non-model overlays like glow still apply).
- **Model-centric pulls** (`models/<name>:<quant>`) set `RequestedModel` via
  metahub-api `resolveModelCentricRuntime`, so the stack fires and composes
  runtime + weights + config.

### Key files
- `pkg/backends/magazine/manipulation.go` — the gate + static-path filter
- `pkg/backends/magazine/model_resolution_engine_test.go` — 3 new tests
  (collection no-inject, model-centric fires stack, config-manip skipped on
  collection) + 2 existing tests updated to set `WithRequestedModel`

### Gotchas
- `scopeManipulationsToModel` returns ALL manipulations when `RequestedModel` is
  empty (single-model-per-profile back-compat), so the **gate** must be checked
  separately — the static-path filter (`model/`/`config/` prefix skip) is what
  actually prevents the leak on collection pulls.
- The config manipulation's flat layer has an **empty rule** (match-all), so it
  would have matched via the static path without the explicit prefix skip.
- `IndexGetByRequest` (multi-descriptor index path) applies manipulations via
  `manipulateIndexInPlace` in metahub-api, which passes `ctx` through — the gate
  is effective there too, not just the single-manifest path.

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

## Session: add-manipulayer-select (2026-06, commit 2881016)

ManipuLayer layer selection via Python-style slice expressions. Lets a
manipulation overlay (e.g. one built `FROM alpine`) skip its own base
layer when applied on top of an already-alpine target, so the base
isn't duplicated. Capability: `manipulation-apply-select`.

### The field's path through the stack
`select` flows YAML (`config.ManipuLayer.Select`) → types
(`types.ManipuLayer.Select`) → graph (`:ManipuLayer {select}` string
prop, coalesced to `""` so pre-change nodes round-trip) → DTO →
metahub-api → metahub-ui chip. Missing/empty `select` = `[:]` (all
layers) — every existing manipulation YAML keeps its semantics.

### Where the logic lives
- **`pkg/utils/slice.go`** — `ParseLayerSlice` (syntax only) +
  `LayerSlice.ApplyIndices`/`ApplyToLayers`. `resolve` translates the
  parsed slice against a concrete length `n`. Refactored into
  `resolveSingle` (single-index branch) + `resolveBound` (shared
  start/end wrap+range-check) to stay under the cognitive-complexity
  gate (was 20, now 5).
- **`pkg/backends/neo/manipulayer.go`** — `ManipuLayerCreate` persists
  the `select` prop. Split into `manipuLayerInsertNode` /
  `manipuLayerLinkManifest` / `manipuLayerLinkRule` to stay under the
  complexity gate (was 22, now 9). `manipuLayerInsertNode` calls
  `ParseLayerSlice` **before** the CREATE so a malformed select never
  produces a node.
- **`pkg/backends/magazine/manipulation.go`** —
  `applyManipuLayerSelect` (line ~700) is the ONLY apply-time caller;
  resolves indices and threads them through `manipulationParts` so
  `projectDiffIDs` slices `RootFS.DiffIDs` with the *same* indices as
  the layer descriptors.

### STRICT semantics (decided this session, diverges from author's original)
The original merged code used **Python clamping** (out-of-range ranges
silently resolved to empty). This was tightened to **strict**:
- Single index out of `[0,n)` → error (`"out of range"`).
- Range bound (start or end) out of `[0,n]` after negative-wrapping →
  error (`"out of range"`).
- Range resolving to empty (`[3:1]`, `[0:0]`, `[5:]` on len 5) →
  error (`"empty layer set"`).
- Negative/zero step (`[::-1]`, `[::0]`) → rejected at parse time.

### Two validation points (and the gap that was closed)
- **Parse time** (`ParseLayerSlice`): syntax/grammar errors. Now also
  invoked on the **write path** (`manipuLayerInsertNode`) — previously
  it was ONLY called at apply time, so malformed YAML could create a
  graph node that only exploded at pull. That gap is closed.
- **Apply time** (`resolve` against real layer count): out-of-range +
  empty-result errors. These are length-dependent so they *cannot*
  move to write time; a syntactically-valid-but-out-of-range select
  can still create a node and fail only at pull. That matches the
  spec (parse-time = syntax only).

### Gotchas
- **Lockstep invariant**: layer descriptors and `RootFS.DiffIDs` MUST
  be sliced with identical indices or docker's pull-time diff check
  rejects the image. `projectDiffIDs` enforces it; `nil` indices mean
  identity/pass-through (DiffIDs untouched).
- Two distinct error *messages* at the boundary: `[6:]` on 5 →
  "out of range" (bound beyond length); `[5:]` on 5 → "empty layer
  set" (in-bounds but selects nothing). Both fail the pull.
- neo tests need a live Neo4j; the negative write-validation test
  (`manipulayer_select_test.go`) asserts *before* any `n.Execute`, so
  it surfaces the parse error regardless of DB state.

## Session: collection-profile-authority (2026-06)

A collection manifest entry's explicit `profile:` field is now the
authority for which profile a manifest is served under.

### Why
`docker pull …/qnib/ftest:latest` under `cpu#zen2` returned
MANIFEST_UNKNOWN even though the CE collection authored
`profile: cpu#zen2` on the zen2 manifest. Root cause: `config.Manifest`
had **no `Profile` field**, and yaml.v3 silently drops unknown keys —
so the authored intent was parsed into nothing. All three ftest
manifests landed under `generic` only; no `cpu#zen2` slot/annotation
was ever created.

### What changed
- `pkg/yaml/config/manifest.go` — added `Profile string
  \`yaml:"profile,omitempty"\``. Empty = pre-change behaviour.
- `pkg/backends/magazine/collection.go` — new
  `Magazine.BindManifestToProfile(repo, tag, profileLabel, mdID)`:
  resolves the profile and mirrors the generic parse wiring
  (`SlotGetCreate` + `SlotAppendPushedManifest` + `LinkManifestToProfile`
  PULLED_BY + `ProfileUnwindRules` + `ProfileInjectDeterministicAnnotations`).
  The inject step stamps the profile's `anno eq` rules (e.g.
  `org.supercontainers.hardware.cpu.target.microarch=zen2`) onto the
  manifest as `HAS_ANNOTATION` edges, which is what
  `ProfileFromManifestIDViaAnnotations` matches at serve time.
  Empty label = no-op; unknown label = hard error ("does not resolve").
- Consumer lives in metahub-admin `cmd/parse_collections.go`: calls
  `BindManifestToProfile` per manifest entry with a declared profile,
  then computes the per-declared-profile index after the manifest loop.

### Design note
The binding logic lives in metalib (not the admin CLI) so it is
unit-testable against live Neo4j and reusable by the planned
`MaterializeManifest` unification (see metahub-admin CONTEXT "Future
Direction"). Tests: `collection_profile_bind_test.go` (inject / empty /
unknown) + `collection_test.go::TestManifestProfileRoundTrip`.

### Gotchas
- **Forward-only.** Binding happens at parse/materialize time. An
  already-stored manifest (e.g. the current ftest zen2 image) will NOT
  gain the binding until the collection is re-parsed. No in-place
  migration command exists — re-run `metahub-admin parse collections`
  after seeding profiles+rules.
- The explicit `profile:` is *additional* to the generic wiring, not a
  replacement: the manifest is still wired under `generic` AND bound to
  its declared profile.
- A secondary bug is out of scope here: bare profile `zen2` (no `#`)
  returns 500 instead of 404 when nothing matches — tracked as a
  metahub-api downstream fix.

---

## Session: waldur-outfitter-integration (2026-08-04)

### OCI error codes now carry their spec status

`utils.ErrorHandlerWithWWW` mapped only `MANIFEST_UNKNOWN` and
`MANIFEST_INVALID`/`INTERNAL_ERROR`; everything else fell through to **HTTP 500**
with an "Unknown error type" warning. `DENIED` has 16 call sites and
`UNAUTHORIZED` 9, so authorization outcomes were reported as server faults — in
one session a quota rejection, a missing profile and a namespace-authz denial all
surfaced as 500, and each time the first assumption was that metahub had broken
rather than that it had said no. skopeo prints the status and nothing else, so
the real reason only appears in the server log.

Now `DENIED → 403`, `UNAUTHORIZED → 401`. `BLOB_UNKNOWN` and `NAME_UNKNOWN` still
fall through (spec says 404) — deliberately left, since they touch the OCI
conformance surface and deserve their own run.

### ORAS artifact push returned an empty MediaType

`StoreManifestUpload` populates a content-leaf (`ID`, `Digest`, `Label`) and then,
for artifacts, **discarded it**:

```go
md, ociMani, e := mag.storeManifestUploadOCIManifest(...)   // md is real
md.Digest, md.Label = ...
if ociMani.IsArtifact() {
    return mag.StoreManifestArtifactUpload(ctx, req, ociMani)   // md thrown away
}
```

`StoreManifestArtifactUpload` re-derived `md` from a graph lookup that misses on
a **first** push, so it returned a zero-value struct with only `ArtifactType`
set — and `WriteWWWHeader` rejects an empty `MediaType`, producing a 500 on a
manifest that had actually stored fine. Only ORAS artifacts hit it (the image
path continues on and populates everything), which is why it presented as
"outfitter pushes are broken". The content-leaf is now threaded through, which
also stops `wireSlotOnPush` receiving `md.ID == 0`.

### Legacy push authz lives here

`ProfileAuthorizedPushHandler` enforces "repo path must start with the username,
else admin". metahub-api now skips it when `registry.enforce_write_ownership` is
on, because it silently overrode the namespace matrix. If you touch it, note the
comment says "profile name" but the code compares `req.User`.
