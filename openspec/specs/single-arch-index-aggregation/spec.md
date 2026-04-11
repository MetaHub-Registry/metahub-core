## ADDED Requirements

### Requirement: Push-time compute trigger for a single `(repo, tag)`
The Magazine API SHALL expose `ComputeManifestlistsForRepoTag(ctx, repo, tag, pusherPrf *types.Profile) error`, a push-time primitive that eagerly injects the pusher profile's deterministic annotations onto every `ManifestData` linked to `(repo, tag)` and then rebuilds all per-profile `ManifestList`s for that `(collection, tag)`.

#### Scenario: Fresh single-arch push with a matching profile
- **GIVEN** a `Profile{Name="generic", Tag="v1"}` with a single rule `{Type="anno", Op="eq", Key="org.metahub.release", Value="stable"}`
- **AND** a `Collection{Repo="library/alpine"}` with `CollectionTag{Tag="3.23"}` and exactly one `ManifestData` (linux/amd64) freshly written by `StoreManifestUpload`
- **WHEN** `Magazine.ComputeManifestlistsForRepoTag(ctx, "library/alpine", "3.23", &genericPrf)` is called
- **THEN** the helper SHALL inject the annotation `org.metahub.release=stable` onto the manifestdata (via `ProfileInjectDeterministicAnnotations`)
- **THEN** the helper SHALL call `RuleEvalAll` so the `Rule-[:TRUE]->Annotation` edges are created
- **THEN** the helper SHALL iterate every profile returned by `ProfileGetAll` and for each build a `CollectionTagProfilePair` for `(Collection, CollectionTag, Profile)` hydrated with the tag's `ManifestData` rows
- **THEN** the helper SHALL call `ComputeManifestlistForCTTP` for each such CTPP, which in turn SHALL produce exactly one new `manifestlists` row (linked to `Collection`, `CollectionTag`, `Profile`) for every profile whose rules match at least one manifestdata
- **THEN** the helper SHALL return a nil error

#### Scenario: Second push of a different architecture to the same tag
- **GIVEN** `library/alpine:3.23` already has a `linux/amd64` manifestdata and a matching `ManifestList` for profile `generic`
- **WHEN** a new `linux/arm64` manifest is pushed to the same tag and the handler calls `ComputeManifestlistsForRepoTag(ctx, "library/alpine", "3.23", &genericPrf)`
- **THEN** the helper SHALL inject `generic`'s deterministic annotations onto the new arm64 manifestdata (not the existing amd64 one)
- **THEN** the helper SHALL rebuild the `generic` CTPP such that `ctpp.Manifests` contains both platforms
- **THEN** `ComputeManifestlistForCTTP` SHALL produce a new `ManifestList` whose `Manifests` array has exactly two entries (`linux/amd64`, `linux/arm64`) with distinct digests
- **THEN** the new `ManifestList` SHALL be linked to the same `Collection`, `CollectionTag`, and `Profile=generic` as the previous one

#### Scenario: Nil pusher profile skips annotation injection
- **GIVEN** a `(repo, tag)` with existing manifestdata rows and any set of profiles in the system
- **WHEN** `ComputeManifestlistsForRepoTag(ctx, repo, tag, nil)` is called
- **THEN** the helper SHALL NOT invoke `ProfileInjectDeterministicAnnotations`
- **THEN** the helper SHALL still run `RuleEvalAll` and rebuild matching indexes against whatever annotations the manifestdata already has
- **THEN** profiles that match the existing annotations SHALL receive their rebuilt index
- **THEN** profiles that do not match any manifestdata in the tag SHALL NOT produce an index (the existing empty-set short-circuit in `ComputeManifestlistForCTTP` applies)

#### Scenario: Non-existent `(repo, tag)` is an error
- **WHEN** `ComputeManifestlistsForRepoTag(ctx, repo, tag, prf)` is called for a `(repo, tag)` that has no `Collection` or `CollectionTag` row
- **THEN** the helper SHALL return a non-nil error naming the missing `(repo, tag)`
- **THEN** no annotations, rules, or manifestlists SHALL be inserted

#### Scenario: Idempotent re-invocation
- **WHEN** `ComputeManifestlistsForRepoTag` is called twice in succession with identical arguments
- **THEN** the second call SHALL produce no new annotations on the manifestdata rows (MERGE semantics on `ManifestDataLinkToAnnotation`)
- **THEN** any `ManifestList` produced by the second call SHALL have the same digest as the first call's output
- **THEN** `StoreManifestListUpload`/`InsertManifestList` SHALL short-circuit on the second call (existing-digest path), producing no duplicate rows
- **THEN** the second call SHALL return a nil error

### Requirement: Deterministic-annotation injection helper on `Database`
The `magazine.Database` interface SHALL expose `ProfileInjectDeterministicAnnotations(ctx, prfID, maniDataID int64) (injected int, skipped int, err error)` with a consistent implementation in both shipped backends.

#### Scenario: Profile with only `anno eq` rules
- **GIVEN** a profile whose rules are exclusively `{Type="anno", Op="eq", Key=k_i, Value=v_i}` for i in [1..N]
- **WHEN** `ProfileInjectDeterministicAnnotations(ctx, prfID, mdID)` is called
- **THEN** the helper SHALL insert or merge each `(k_i, v_i)` `Annotation` node and link each to the manifestdata via `ManifestDataLinkToAnnotation`
- **THEN** the call SHALL return `(injected == N, skipped == 0, nil)`

#### Scenario: Profile with a mix of rule types and ops
- **GIVEN** a profile with three rules: `{anno eq k1=v1}`, `{anno re k2=^v.*$}`, `{plat eq arch=amd64}`
- **WHEN** the helper is called
- **THEN** only the `{anno eq k1=v1}` rule SHALL be injected as an annotation link
- **THEN** the other two rules SHALL be skipped
- **THEN** the helper SHALL emit one debug log line per skipped rule naming the rule's label, type, and op
- **THEN** the call SHALL return `(injected == 1, skipped == 2, nil)`

#### Scenario: Profile with no deterministic rules
- **GIVEN** a profile whose rules are exclusively non-`anno eq` (platform rules, regex rules, etc.)
- **WHEN** the helper is called
- **THEN** the call SHALL return `(injected == 0, skipped == <count>, nil)` — not an error
- **THEN** no annotation nodes or links SHALL be created

#### Scenario: Idempotent injection
- **GIVEN** a profile with one `anno eq` rule and a manifestdata that has already been injected with that annotation
- **WHEN** `ProfileInjectDeterministicAnnotations` is called a second time
- **THEN** the existing annotation node SHALL be reused (`AnnotationGetInsert` MERGE semantics)
- **THEN** the existing `HAS_ANNOTATION` link SHALL be reused (the underlying graph MERGE on the relationship)
- **THEN** the call SHALL return `(injected == 1, skipped == 0, nil)` — the helper does not distinguish "freshly linked" from "already linked"; the important invariant is that no duplicate rows/edges are created

### Requirement: `ManifestDataGetByCollectionTag` is part of the `Database` interface
The `magazine.Database` interface SHALL expose `ManifestDataGetByCollectionTag(ctx, collectionID, tagID int64) ([]types.ManifestData, error)` and both shipped backends SHALL implement it.

#### Scenario: Tag with multiple manifestdata rows
- **GIVEN** a `(Collection, CollectionTag)` pair with three linked `ManifestData` rows across two architectures
- **WHEN** `ManifestDataGetByCollectionTag(ctx, colID, tagID)` is called
- **THEN** the call SHALL return a slice containing all three manifestdata rows
- **THEN** each entry SHALL have its `ID`, `Digest`, `MediaType`, and `Platform` populated

#### Scenario: Tag with no manifestdata
- **WHEN** the call is made for a `(colID, tagID)` pair that has no `HAS_MANIFESTDATA` links
- **THEN** the call SHALL return an empty slice and a nil error (NOT an error)
