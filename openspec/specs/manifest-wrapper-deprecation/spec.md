# manifest-wrapper-deprecation Specification

## Purpose
TBD - created by archiving change deprecate-manifest-wrapper. Update Purpose after archive.
## Requirements
### Requirement: Slot/leaf-based push reshape entrypoint

The `magazine` reshape pipeline SHALL expose an entrypoint that reshapes a pushed Docker v2 manifest into OCI v1 per profile **without** requiring the caller to supply a `types.ManifestWrapper` argument. The entrypoint SHALL accept the content-leaf identity of the pushed manifest (a content-leaf `types.Manifest`, as returned by `StoreManifestUpload`) and SHALL perform the same per-profile reshaping previously reachable via `ComputeManifest(ctx, types.ManifestWrapper)`.

The wrapper-typed `ComputeManifest(ctx, types.ManifestWrapper)` method SHALL be retained until the read-side migration (see the `migrate-wrapper-read-cypher` change) rekeys reshaping fully onto the Slot/content-leaf, after which it and the `types.ManifestWrapper` type SHALL be removed.

> Note: as shipped, the leaf-based entrypoint resolves the wrapper identity internally from the content-leaf (an implementation detail) so that the **caller** — and in particular metahub-api — no longer references `:ManifestWrapper`. Eliminating that internal resolution (so reshaping reads no wrapper at all) is owned by the `migrate-wrapper-read-cypher` change.

#### Scenario: Push reshaping no longer needs a caller-supplied wrapper
- **WHEN** a Docker v2 manifest is pushed and stored, yielding a content-leaf `:Manifest`
- **THEN** the reshape pipeline SHALL be invokable with the content-leaf identity alone
- **AND** the resulting per-profile OCI v1 manifests SHALL be byte-identical to those produced by the previous wrapper-based `ComputeManifest`
- **AND** the caller SHALL NOT fetch or pass a `types.ManifestWrapper`

### Requirement: Removal of dead and test-only ManifestWrapper surface

The legacy `ManifestWrapper*` API surface that has no production caller SHALL be removed from the `Neo` backend, the `magazine` backend, and the `magazine.Database` interface. This SHALL include at minimum: `HeadManistNotFoundForProfile` and `ManifestWrapperGetByRepoTag`; the OS-annotation helpers `ManifestWrapperGetOS`, `ManifestWrapperUpsertOS`, `ManifestWrapperUpsertOSVersion`, `ManifestWrapperUpsertOSID`, and `ManifestWrapperGetAllSystemAnnos`; and the profile helpers `ProfileFromManifestWrapperIDViaAnnotations` and `ProfileFromManifestWrapperIDViaPlatform`.

Removal SHALL NOT change observable behaviour, because OS annotations already reside on the content-leaf and the listed methods have no production callers.

#### Scenario: Dead method removal keeps the build and tests green
- **WHEN** the dead and test-only `ManifestWrapper*` methods and their tests are deleted
- **THEN** the metalib module SHALL compile
- **AND** the remaining test suite SHALL pass without referencing the removed methods

#### Scenario: OS annotations remain readable from the content-leaf
- **WHEN** a manifest with OS annotations is queried after the OS-annotation wrapper helpers are removed
- **THEN** the OS id and version SHALL still be resolvable from the content-leaf annotations

### Requirement: Test fixtures wire the Slot-centred graph

Test setup helpers SHALL wire the Slot-centred graph for manifest fixtures: a `:Slot` for the `(repo, tag, profile)` triple plus a `:HAS_PUSHED` edge to the content-leaf `:Manifest` (via `SlotGetCreate` + `SlotAppendPushedManifest`). This makes fixtures exercise the Slot-centred model alongside any legacy wrapper attachment.

> Note: removing wrapper-node *creation* from the fixtures (so that no
> `:ManifestWrapper` node exists after setup) is coupled to migrating the
> remaining wrapper read Cypher and is owned by the `migrate-wrapper-read-cypher`
> change. The one-time `MATCH (m:ManifestWrapper) DETACH DELETE m` migration that
> purges persisted wrapper nodes from existing databases is owned by the Phase 5
> type/file removal change.

#### Scenario: Fixtures create a Slot and content-leaf linked by HAS_PUSHED
- **WHEN** a test sets up a repo/tag/profile fixture with a manifest
- **THEN** the fixture SHALL create a `:Slot` and a content-leaf `:Manifest`
- **AND** the `:Slot` SHALL be linked to the content-leaf via `:HAS_PUSHED`

