## Purpose

Defines the slot-centred graph model for metalib, where `:Slot` nodes serve as the primary anchor for per-`(repo, tag, profile)` state, replacing the legacy `:Collection`/`:ManifestData`/`:ManifestList`/`:Manifest` wrapper model.

## Requirements

### Requirement: Slot node as primary (repo, tag, profile) anchor

The metalib graph model SHALL introduce a `:Slot` node label as the primary anchor for per-`(repo, tag, profile)` state. Every distinct triple of repo name, tag name, and profile label SHALL correspond to exactly one Slot node, enforced by a Neo4j UNIQUE constraint on the compound key `(s.repo, s.tag, s.profile)`.

Slot nodes SHALL carry the triple as node properties (`repo`, `tag`, `profile`) as the source of truth for the compound UNIQUE constraint, and SHALL additionally be linked to the corresponding string-identity nodes via denormalised join edges:

- `(s:Slot)-[:OF_REPO]->(r:Repo {name: s.repo})`
- `(s:Slot)-[:OF_TAG]->(t:Tag {name: s.tag})`
- `(s:Slot)-[:OF_PROFILE]->(p:Profile {label: s.profile})`

Slot nodes SHALL additionally carry a `created_at` timestamp property recording when the triple was first seen.

#### Scenario: Slot creation is idempotent
- **WHEN** `SlotGetCreate(ctx, "qnib/alpine", "latest", "generic")` is called twice
- **THEN** exactly one Slot node with those three property values SHALL exist in the graph
- **AND** the second call SHALL return the same Slot node ID as the first
- **AND** no duplicate `OF_REPO`, `OF_TAG`, or `OF_PROFILE` edges SHALL exist

#### Scenario: Slot compound uniqueness is enforced at the database level
- **WHEN** two separate transactions attempt to create Slots with identical `(repo, tag, profile)` triples concurrently
- **THEN** exactly one creation SHALL succeed
- **AND** the other SHALL either return the same Slot ID (via MERGE idempotency) or fail with a constraint violation that the caller can retry

#### Scenario: Different triples produce different Slots
- **WHEN** `SlotGetCreate` is called with `(qnib/alpine, latest, generic)` and then with `(qnib/alpine, latest, cpu#zen2)`
- **THEN** two distinct Slot nodes SHALL exist, each with the appropriate `OF_PROFILE` join edge

#### Scenario: Slot creation MERGEs string-identity nodes
- **WHEN** `SlotGetCreate(ctx, "new/repo", "new-tag", "new-profile")` is called and no `:Repo`, `:Tag`, or `:Profile` matching those identifiers exists
- **THEN** the three string-identity nodes SHALL be created via MERGE
- **AND** the Slot's three join edges SHALL point at them

### Requirement: Delete the legacy :Collection, :ManifestData, :ManifestList, and :Manifest (wrapper) node labels

The metalib graph model SHALL NOT contain any node with the label `:Collection`, `:ManifestData`, `:ManifestList`, or the legacy metahub-specific `:Manifest` wrapper (which was keyed on `label = "host/repo:tag"` and linked via `[:MANIFEST_DERIVED_FROM]`). Their roles SHALL be taken over by `:Repo`, `:Manifest` (digest-keyed), `:Index`, and `:Slot` respectively.

The schema bootstrap in `Main.neo4jSetupSchema` SHALL drop the old UNIQUE constraints on these labels if present, and SHALL create the new constraints on the replacement labels.

#### Scenario: Fresh database bootstrap creates only the new labels
- **WHEN** metalib is started against an empty Neo4j instance
- **THEN** the schema setup SHALL create UNIQUE constraints on `(:Repo {name})`, `(:Manifest {digest})`, `(:Index {digest})`, and `(:Slot {repo, tag, profile})`
- **AND** no constraints on `:Collection`, `:ManifestData`, `:ManifestList`, or the legacy `:Manifest` wrapper SHALL be created

#### Scenario: Legacy constraint cleanup runs idempotently
- **WHEN** the schema setup runs against a database that previously had the legacy constraints
- **THEN** the `DROP CONSTRAINT ... IF EXISTS` statements SHALL execute without error
- **AND** the new constraints SHALL be created on their replacement labels

### Requirement: Rename :Collection to :Repo

What the current model calls a `:Collection` node SHALL be renamed to `:Repo`. The `name` property carries the `<namespace>/<name>` path component of the registry URL (e.g. `qnib/alpine`). A UNIQUE constraint on `(:Repo {name})` SHALL be enforced.

All metalib helper functions referring to Collections SHALL be renamed to their Repo equivalents:

- `CollectionGetCreateByRepoTag` → removed (replaced by `SlotGetCreate` and `RepoGetCreate`)
- `Collection` Go type → renamed to `Repo`
- `collection.go` file → renamed to `repo.go`
- `Collection.ColTag` field → removed (tag ownership moves to the `HAS_TAG` edge from Repo)

#### Scenario: Repo node is created on first Slot creation for a new repo
- **WHEN** `SlotGetCreate(ctx, "brand/new-repo", "v1", "generic")` is called and no `:Repo {name:"brand/new-repo"}` exists
- **THEN** a new `:Repo` node SHALL be MERGEd with `name = "brand/new-repo"`

#### Scenario: Repo reuse across tags
- **WHEN** Slots for `(repo=same/repo, tag=v1)` and `(repo=same/repo, tag=v2)` are created
- **THEN** exactly one `:Repo {name: "same/repo"}` node SHALL exist
- **AND** both Slots SHALL have their `OF_REPO` edge pointing at the same Repo node

### Requirement: Rename :ManifestData to :Manifest

What the current model calls `:ManifestData` SHALL be renamed to `:Manifest`. These nodes represent single-arch OCI manifests (image or artifact leaves) and are content-addressed by digest. A UNIQUE constraint on `(:Manifest {digest})` SHALL be enforced.

The legacy `:Manifest` wrapper node (the one that was keyed on `label = "host/repo:tag"`) is deleted entirely — its role is taken by `:Slot`.

The `types.ManifestData` Go type SHALL be renamed to `types.Manifest`. The existing `types.Manifest` type (which was a stub for the wrapper) SHALL be deleted.

#### Scenario: Manifest node is content-addressable
- **WHEN** two push requests arrive with byte-identical manifest bodies
- **THEN** exactly one `:Manifest` node with that digest SHALL exist in the graph
- **AND** both pushes SHALL MERGE the same node (idempotent)

#### Scenario: Manifest node survives multi-arch replacement
- **WHEN** a Manifest with digest A is pushed to `qnib/alpine:latest` under profile `generic`
- **AND** a Manifest with digest B is later pushed to the same `qnib/alpine:latest` under profile `generic`
- **THEN** both Manifest nodes SHALL exist in the graph
- **AND** the Slot for `(qnib/alpine, latest, generic)` SHALL have `HAS_PUSHED` edges to both
- **AND** neither Manifest node SHALL have been deleted or orphaned by the subsequent push

### Requirement: Rename :ManifestList to :Index

What the current model calls `:ManifestList` SHALL be renamed to `:Index`. These nodes represent OCI image indexes (or Docker manifest lists) — collections of manifest descriptors, content-addressed by digest. A UNIQUE constraint on `(:Index {digest})` SHALL be enforced.

`:Index` nodes SHALL carry an `origin` string property with values `"computed"` (created by `ComputeIndexForRepoTag`) or `"client-pushed"` (created by `StoreIndexUpload` from client-supplied bytes). First-writer-wins semantics via `ON CREATE SET i.origin = X` — subsequent MERGEs SHALL leave `origin` unchanged.

The `types.ManifestList` Go type SHALL be renamed to `types.Index`.

#### Scenario: Index node carries origin label
- **WHEN** a client pushes an Index via `StoreIndexUpload`
- **THEN** the created Index node SHALL have `origin = "client-pushed"`

#### Scenario: Computed index carries origin label
- **WHEN** `ComputeIndexForRepoTag` produces a new Index
- **THEN** the created Index node SHALL have `origin = "computed"`

#### Scenario: Origin is first-writer-wins on digest conflict
- **WHEN** a computed Index happens to share the same digest as a previously client-pushed Index
- **THEN** the MERGE SHALL match the existing node and leave its `origin` property unchanged
- **AND** the compute path SHALL NOT overwrite `origin` to `"computed"`

### Requirement: Slot push history via HAS_PUSHED edges

Every `:Manifest` push SHALL produce a `(Slot)-[:HAS_PUSHED {timestamp}]->(Manifest)` edge on the pusher's Slot. These edges are append-only and SHALL NOT be deleted or garbage-collected by any code path in this change.

The Slot-scoped location of HAS_PUSHED avoids the edge-explosion problem that would occur if push history hung directly on `:Tag` (which is a global deduplicated string shared across every repo using that tag name).

#### Scenario: Each push creates a HAS_PUSHED edge
- **WHEN** three separate Manifest pushes occur to `(qnib/alpine, latest, generic)`
- **THEN** the Slot for that triple SHALL have exactly three `HAS_PUSHED` edges, one to each Manifest
- **AND** each edge SHALL carry a distinct `timestamp` property

#### Scenario: Idempotent re-push creates a new HAS_PUSHED edge
- **WHEN** the same Manifest (same digest) is pushed twice
- **THEN** two `HAS_PUSHED` edges SHALL exist from the Slot to the same Manifest node
- **AND** they SHALL have different `timestamp` values

#### Scenario: HAS_PUSHED is scoped per Slot
- **WHEN** the same Manifest is pushed to `(qnib/alpine, latest, generic)` and `(qnib/alpine, latest, cpu#zen2)`
- **THEN** two distinct Slots exist, each with its own `HAS_PUSHED` edge to the Manifest
- **AND** the two edges have independent `timestamp` values

### Requirement: Slot POINTS_TO edge as current index pointer

Each Slot SHALL have at most one `(Slot)-[:POINTS_TO]->(Index)` edge at any time, representing the current index served for tag pulls of that `(repo, tag, profile)` triple. `ComputeIndexForRepoTag` SHALL maintain this invariant atomically: when it produces a new Index that differs from the current POINTS_TO target, the old target SHALL be moved to `HAS_HISTORY` with a timestamp, and the new POINTS_TO edge SHALL be created, in a single Neo4j transaction.

If the newly-computed Index has the same digest as the current POINTS_TO target, the compute SHALL short-circuit and leave both POINTS_TO and HAS_HISTORY unchanged (idempotent).

#### Scenario: POINTS_TO is singular per Slot
- **WHEN** `ComputeIndexForRepoTag` produces a new Index for a Slot that already has a POINTS_TO target
- **THEN** after the compute, the Slot SHALL have exactly one outgoing POINTS_TO edge
- **AND** the previous target SHALL have a HAS_HISTORY edge to the Slot with a timestamp

#### Scenario: Idempotent recompute does not touch the graph
- **WHEN** `ComputeIndexForRepoTag` is called twice in succession with no new pushes in between
- **THEN** the second call SHALL detect the digest match and short-circuit before writing
- **AND** the Slot's POINTS_TO edge SHALL be identical to its pre-call state
- **AND** no new HAS_HISTORY edge SHALL be created

#### Scenario: First push creates POINTS_TO without archiving
- **WHEN** the first Manifest is pushed to a never-before-seen `(repo, tag, profile)` triple
- **THEN** a new Slot is created with zero initial POINTS_TO
- **AND** `ComputeIndexForRepoTag` creates a POINTS_TO edge to the new Index
- **AND** no HAS_HISTORY edge is created (nothing to archive)

### Requirement: Slot HAS_HISTORY edges preserve audit trail without granting pull access

`(Slot)-[:HAS_HISTORY {timestamp}]->(Index)` edges SHALL be append-only, SHALL never be deleted, and SHALL NOT grant pull-reachability for Manifests or Blobs referenced by the historical Index. Only `POINTS_TO` and `HAS_PUSHED_INDEX` grant pull reachability.

The pull handler SHALL NOT walk `HAS_HISTORY` edges when resolving tag pulls or digest pulls. The WebUI MAY walk these edges to display historical state as metadata.

#### Scenario: Historical index is not served on tag pull
- **WHEN** a Slot has POINTS_TO → Index A (current) and HAS_HISTORY → Index B (archived)
- **AND** a tag pull arrives for the Slot's `(repo, tag)` with the Slot's profile
- **THEN** the pull SHALL serve Index A
- **AND** the pull SHALL NOT consider Index B

#### Scenario: Historical manifest is not reachable via digest pull
- **WHEN** Index B (archived in HAS_HISTORY) references Manifest X via `[:REFERENCES]`
- **AND** Manifest X is not referenced by any current POINTS_TO or HAS_PUSHED_INDEX target
- **AND** a digest pull arrives for Manifest X under the same profile
- **THEN** the pull SHALL return 404
- **UNLESS** Manifest X is also reachable via `HAS_PUSHED` from the Slot (push history is separate from index history)

#### Scenario: Historical blob is not reachable via blob pull
- **WHEN** a Blob is referenced only by a Manifest that exists only in HAS_HISTORY Indexes
- **AND** a blob pull arrives under the same profile
- **THEN** the pull SHALL return 404 for the blob reachability chain
- **AND** the pull MAY return 200 if the blob is reachable via `Profile -[:UPLOADED]-> Blob` attribution (pre-manifest upload window)

### Requirement: Slot HAS_PUSHED_INDEX preserves client-pushed index bytes

When a client pushes an `:Index` directly (via `handleOCIManifestListPut` / `StoreIndexUpload`), the metalib backend SHALL:

1. Store the client's raw index bytes verbatim in minio keyed by the client-computed digest.
2. MERGE an `:Index {digest, origin: "client-pushed"}` node.
3. Ensure the member `:Manifest` nodes referenced by the index exist (return an OCI `MANIFEST_BLOB_UNKNOWN` error to the client if any are missing).
4. MERGE `:REFERENCES {order, platform}` edges from the Index to each member Manifest.
5. Attach the Index to the pushing profile's Slot via `(Slot)-[:HAS_PUSHED_INDEX {timestamp}]->(Index)`.
6. Invoke `ComputeIndexForRepoTag` to recompute per-profile POINTS_TO targets across all profiles in the graph.

`HAS_PUSHED_INDEX` edges SHALL grant pull reachability at the same scope as `POINTS_TO` — i.e. Manifests and Blobs reachable via `HAS_PUSHED_INDEX -> REFERENCES -> HAS_LAYER/HAS_CONFIG` SHALL be servable on digest pulls under the pushing profile's context.

#### Scenario: Client-pushed index bytes are served on direct-digest pull
- **WHEN** a client pushes an Index with computed digest `sha256:AABB` via `oci/generic` login
- **THEN** `GET /v2/qnib/alpine/manifests/sha256:AABB` with the pusher's profile SHALL return HTTP 200
- **AND** the response body SHALL be the byte-identical content the client pushed

#### Scenario: Client-pushed index triggers per-profile recompute
- **WHEN** a client pushes an Index referencing Manifests A (amd64) and B (arm64)
- **AND** the graph contains profiles `generic` (matches both) and `cpu#zen2` (matches neither because no zen2 annotation)
- **THEN** after the push, `generic`'s Slot for `(repo, tag, generic)` SHALL have a POINTS_TO target (computed or equal to client's digest)
- **AND** `cpu#zen2`'s Slot for `(repo, tag, cpu#zen2)` SHALL have no POINTS_TO edge or an explicit empty marker

#### Scenario: Missing member manifest rejects the push
- **WHEN** a client pushes an Index referencing a Manifest digest not present in the graph
- **THEN** the push SHALL fail with HTTP 400 and OCI error code `MANIFEST_BLOB_UNKNOWN`
- **AND** no `:Index` node SHALL be created

### Requirement: Index REFERENCES edges preserve descriptor order and platform hints

`(Index)-[:REFERENCES {order, platform}]->(Manifest)` edges SHALL carry:

- `order`: zero-indexed integer position matching the descriptor's position in the index JSON's `manifests` array
- `platform`: optional map mirroring the OCI `Descriptor.Platform` field (`{os, architecture, variant, os.version, os.features}`). This is preserved even when the target Manifest's own `HAS_PLATFORM` edge would provide the same information, because the OCI spec allows a client to override descriptor platforms.

The pull handler SHALL serialise the Index body by traversing REFERENCES edges in `order` and constructing the descriptor array from the target Manifest's digest+mediaType+size plus the REFERENCES edge's `platform` property.

#### Scenario: Descriptor order is preserved on pull
- **WHEN** an Index was pushed with amd64 at order 0 and arm64 at order 1
- **AND** the Index is pulled via its digest
- **THEN** the returned body's `manifests` array SHALL have the amd64 descriptor at index 0 and arm64 at index 1

#### Scenario: Descriptor platform override is preserved
- **WHEN** a client pushes an Index where the descriptor's platform differs from the referenced Manifest's canonical platform (e.g. a custom `os.version` hint)
- **THEN** the pull SHALL return the client's original platform override
- **AND** the pull SHALL NOT substitute the Manifest's canonical platform

### Requirement: Pull-side tag resolution walks Slot → POINTS_TO → Index

`GET /v2/<repo>/manifests/<tag>` SHALL resolve the tag by walking:

```
MATCH (r:Repo {name:$repo})-[:HAS_TAG]->(t:Tag {name:$tag})
MATCH (s:Slot {repo:$repo, tag:$tag, profile:$profile})-[:POINTS_TO]->(x)
WHERE x:Index OR x:Manifest
RETURN x
```

If no matching Slot exists, the handler SHALL return HTTP 404 with OCI error code `MANIFEST_UNKNOWN`. The handler SHALL NOT fall back to the profile's `cpu:noop`-style defaults or any other profile — the profile resolution from the auth middleware is authoritative.

The returned body SHALL be fully hydrated: for an `:Index` target the `manifests` array SHALL be populated with descriptor entries built from the `[:REFERENCES]` edges.

#### Scenario: Tag pull with matching Slot returns hydrated index
- **WHEN** `GET /v2/qnib/alpine/manifests/latest` is called with profile `generic`
- **AND** a Slot for `(qnib/alpine, latest, generic)` has a POINTS_TO edge to a computed multi-arch Index
- **THEN** the response body SHALL have mediaType `application/vnd.oci.image.index.v1+json`
- **AND** the `manifests` array SHALL contain descriptor entries matching the REFERENCES edges in order
- **AND** the `manifests` array SHALL NOT be null or empty

#### Scenario: Tag pull with no matching Slot returns 404
- **WHEN** `GET /v2/qnib/alpine/manifests/latest` is called with profile `cpu#zen2`
- **AND** no Slot for `(qnib/alpine, latest, cpu#zen2)` exists (because no pushed content matched zen2's rules)
- **THEN** the response SHALL be HTTP 404 with OCI error code `MANIFEST_UNKNOWN`

### Requirement: Pull-side digest resolution walks reachability from Slot

`GET /v2/<repo>/manifests/<digest>` SHALL resolve the digest by first finding the node (Index or Manifest) with the matching digest, then asserting reachability from a Slot matching the requesting profile's triple:

For Index digests:
```
MATCH (i:Index {digest:$digest})
MATCH (s:Slot {repo:$repo, profile:$profile})-[:POINTS_TO|HAS_PUSHED_INDEX]->(i)
RETURN i LIMIT 1
```

For Manifest digests:
```
MATCH (m:Manifest {digest:$digest})
OPTIONAL MATCH ref = (s1:Slot {repo:$repo, profile:$profile})-[:POINTS_TO|HAS_PUSHED_INDEX]->(:Index)-[:REFERENCES]->(m)
OPTIONAL MATCH push = (s2:Slot {repo:$repo, profile:$profile})-[:HAS_PUSHED]->(m)
WITH m, ref, push
WHERE ref IS NOT NULL OR push IS NOT NULL
RETURN m LIMIT 1
```

HAS_HISTORY edges SHALL NOT be walked for reachability.

#### Scenario: Digest pull for current index succeeds
- **WHEN** a Slot has POINTS_TO → Index with digest D
- **AND** `GET /v2/<repo>/manifests/D` arrives with the Slot's profile
- **THEN** the handler SHALL return the Index's bytes (serialised fresh if computed, verbatim if client-pushed)

#### Scenario: Digest pull for historical index returns 404
- **WHEN** a Slot has HAS_HISTORY → Index with digest H (but no POINTS_TO to it)
- **AND** `GET /v2/<repo>/manifests/H` arrives with the Slot's profile
- **THEN** the handler SHALL return HTTP 404

#### Scenario: Digest pull for manifest referenced by current index succeeds
- **WHEN** a Slot's current Index references Manifest M via `[:REFERENCES]`
- **AND** `GET /v2/<repo>/manifests/M.digest` arrives with the Slot's profile
- **THEN** the handler SHALL return the Manifest

#### Scenario: Digest pull for manifest reachable via HAS_PUSHED succeeds even without current index
- **WHEN** a Manifest was pushed (so `HAS_PUSHED` exists) but no current Index references it (e.g. a profile's rules filtered it out of every computed index)
- **AND** `GET /v2/<repo>/manifests/<that-manifest-digest>` arrives with the pushing profile
- **THEN** the handler SHALL return the Manifest via the HAS_PUSHED reachability path

### Requirement: Pull-side blob resolution walks reachability from Slot with UPLOADED fallback

`GET /v2/<repo>/blobs/<digest>` SHALL resolve the blob by walking the transitive reachability chain from the Blob backwards through `HAS_LAYER`/`HAS_CONFIG` to Manifest, then `REFERENCES` backwards to Index, then `POINTS_TO`/`HAS_PUSHED_INDEX` backwards to a Slot matching the requesting profile's triple. If no such path exists, the handler SHALL fall back to checking `(:Profile {label:$profile})-[:UPLOADED {repo:$repo}]->(:Blob {digest:$digest})` for the pre-manifest upload window.

Either reachability path grants pull access. HAS_HISTORY SHALL NOT be walked.

#### Scenario: Blob reachable via current index path returns 200
- **WHEN** a Blob is referenced as a layer of a Manifest that is REFERENCES'd by a POINTS_TO Index on the requesting Slot
- **THEN** `GET /v2/<repo>/blobs/<digest>` SHALL return HTTP 200 with the blob bytes

#### Scenario: Blob reachable via UPLOADED attribution returns 200
- **WHEN** a Blob has a `(:Profile {label:$profile})-[:UPLOADED {repo:$repo}]->(:Blob)` edge
- **AND** no Manifest yet references the Blob (upload-without-manifest window)
- **THEN** `GET /v2/<repo>/blobs/<digest>` SHALL return HTTP 200

#### Scenario: Blob reachable only via historical index returns 404
- **WHEN** a Blob is referenced only by a Manifest in a HAS_HISTORY Index (not in any current POINTS_TO or HAS_PUSHED_INDEX)
- **AND** no UPLOADED attribution exists for the requesting profile
- **THEN** `GET /v2/<repo>/blobs/<digest>` SHALL return HTTP 404

### Requirement: Profile UPLOADED edges attribute blobs at upload time

`StoreBlobUpload` SHALL create a `(:Profile {label})-[:UPLOADED {repo, timestamp}]->(:Blob {digest})` edge on every blob upload. The `repo` property comes from the URL path of `POST /v2/<repo>/blobs/uploads/`. The `timestamp` is the upload completion time.

UPLOADED edges SHALL be append-only. Multiple uploads of the same blob under different profiles or different repos SHALL produce independent edges.

#### Scenario: Blob upload creates UPLOADED edge
- **WHEN** a profile `generic` uploads a blob to `/v2/qnib/alpine/blobs/uploads/...` successfully
- **THEN** a `(:Profile {label:"generic"})-[:UPLOADED {repo:"qnib/alpine", timestamp: <now>}]->(:Blob {digest: <uploaded>})` edge SHALL exist in the graph

#### Scenario: Cross-profile upload creates independent edges
- **WHEN** the same blob digest is uploaded by profile `generic` to `qnib/alpine` and later by profile `cpu#zen2` to `qnib/alpine`
- **THEN** two UPLOADED edges SHALL exist, one per profile
- **AND** both edges SHALL have their own `timestamp`

#### Scenario: Cross-repo upload creates independent edges
- **WHEN** the same profile uploads the same blob digest to `qnib/alpine` and then to `qnib/debian`
- **THEN** two UPLOADED edges SHALL exist, each with its respective `repo` property

### Requirement: Rule shortcut taxonomy as explicit invariant

Each `:Rule` node SHALL have `[:TRUE]` edges to nodes of exactly one label type, determined by its `(type, op)` combination:

| Rule type | Op | Shortcut target |
|-----------|------|-------|
| `plat` | `eq` | `:Platform` |
| `anno` | `eq` | `:Annotation` |
| `anno` | `ne` | `:Annotation` |
| `anno` | `re` | `:Annotation` |
| `anno` | `null` | `:Manifest` (direct, no shortcut) |

No `:Rule` node SHALL have `:TRUE` edges to more than one node label at a time. No other rule type / op combinations are defined by this change.

`RuleEvalAll` SHALL enforce this invariant when materialising edges. Test fixtures SHALL include an `assertRuleShortcutInvariants(t, db)` helper that verifies:

1. Every `plat eq` rule with matching Platforms has `[:TRUE]` edges to `:Platform` nodes only.
2. Every `anno eq`/`ne`/`re` rule with matching Annotations has `[:TRUE]` edges to `:Annotation` nodes only.
3. Every `anno null` rule with matching Manifests has `[:TRUE]` edges to `:Manifest` nodes only.
4. No `:Rule` node has `:TRUE` edges to more than one label type.

#### Scenario: Platform rule shortcut edge type
- **WHEN** `RuleEvalAll` processes a rule with `type="plat", op="eq", key="os", value="linux"`
- **AND** at least one `:Platform {os:"linux"}` node exists in the graph
- **THEN** the rule SHALL have at least one `[:TRUE]->(:Platform)` edge
- **AND** the rule SHALL have no `[:TRUE]->(:Annotation)` or `[:TRUE]->(:Manifest)` edge

#### Scenario: Annotation-eq rule shortcut edge type
- **WHEN** `RuleEvalAll` processes a rule with `type="anno", op="eq", key="stage", value="dev"`
- **AND** at least one `:Annotation {key:"stage", value:"dev"}` node exists
- **THEN** the rule SHALL have at least one `[:TRUE]->(:Annotation)` edge
- **AND** the rule SHALL have no `[:TRUE]->(:Platform)` or `[:TRUE]->(:Manifest)` edge

#### Scenario: Annotation-null rule direct-manifest edge type
- **WHEN** `RuleEvalAll` processes a rule with `type="anno", op="null", key="org.supercontainers.hardware.cpu.*"`
- **AND** at least one `:Manifest` node has no `:HAS_ANNOTATION` edges matching the key pattern
- **THEN** the rule SHALL have `[:TRUE]` edges directly to those Manifest nodes
- **AND** the rule SHALL have no `[:TRUE]->(:Platform)` or `[:TRUE]->(:Annotation)` edge

#### Scenario: assertRuleShortcutInvariants helper fails on mixed edges
- **WHEN** a test artificially creates a rule with both `[:TRUE]->(:Platform)` and `[:TRUE]->(:Annotation)` edges
- **AND** `assertRuleShortcutInvariants(t, db)` is called
- **THEN** the test helper SHALL fail with an error message identifying the offending rule

### Requirement: Rule getter API rename and union

The four rule getters in the neo backend and the `magazine.Database` interface SHALL be renamed for symmetry and clarity:

- `ManifestDataGetByPlatformRule` → `ManifestsMatchingPlatformRule(ctx, rule) ([]Manifest, error)`
- `ManifestDataGetByAnnotationRule` → `ManifestsMatchingAnnotationRule(ctx, rule) ([]Manifest, error)`
- `ManifestDataGetByDirectRule` → `ManifestsMatchingDirectRule(ctx, rule) ([]Manifest, error)`
- `ManifestDataGetByRule` → `ManifestsMatchingRule(ctx, rule) ([]Manifest, error)` (umbrella)

All three leaf getters SHALL walk the cypher shape:

```
MATCH (r:Rule)-[:TRUE]->(<shortcut-target>)
...path to (m:Manifest)...
MATCH (m)-[:HAS_PLATFORM]->(p:Platform)
MATCH (m)-[:ORIGINAL]->(u:Upstream)
RETURN <manifestReturn>, <platformReturn>, <upstreamReturn>
```

with the `<shortcut-target>` varying per flavour (Platform / Annotation / direct Manifest).

The umbrella `ManifestsMatchingRule` SHALL call all three leaf getters and return the union of their results, de-duplicated by Manifest ID.

#### Scenario: Umbrella getter returns union of all three flavours
- **WHEN** a graph contains a Manifest reachable by both a `plat eq` rule and an `anno eq` rule
- **AND** `ManifestsMatchingRule` is called with one of those rules
- **THEN** the returned slice SHALL contain the Manifest exactly once
- **AND** the three leaf getters SHALL each be invoked

#### Scenario: Direct-rule getter returns manifests for null rules only
- **WHEN** the rule is an `anno null` rule
- **AND** `ManifestsMatchingDirectRule` is called
- **THEN** the returned slice SHALL contain only manifests reachable via `(Rule)-[:TRUE]->(Manifest)` direct edges
- **AND** the returned slice SHALL NOT include manifests reachable only via Platform or Annotation shortcuts

### Requirement: ComputeIndexForRepoTag replaces ComputeManifestlistsForRepoTag

The `magazine.Magazine.ComputeManifestlistsForRepoTag` method SHALL be renamed to `ComputeIndexForRepoTag`. Its semantics change to operate against the new model:

1. Fetch all Manifests for `(repo, tag)` via `(Slot)-[:HAS_PUSHED]->(Manifest)` union across every Slot with matching `repo` and `tag`. As a fallback for content predating the first Slot creation (e.g. during live migration of an existing deployment — out of scope for this change but kept for robustness), fall back to `(Manifest)-[:ORIGINAL]->(Upstream {repo, tag})`.
2. If no manifests are found, return `nil` (no error — nothing to compute is not a failure).
3. If `pusherPrf != nil`, inject the pusher's deterministic annotations onto each manifest.
4. Run `RuleEvalAll`.
5. Build an `ExprRes` by iterating every rule and calling `ManifestsMatchingRule`.
6. For every Profile in `ProfileGetAll`:
   a. `SlotGetCreate(ctx, repo, tag, profile.Label)`.
   b. Filter ExprRes by the profile's rule expression → candidate Manifests.
   c. If empty, skip this Slot (no POINTS_TO update).
   d. Compute the OCI Index body from the candidate Manifests (existing `CreateOCIv1ForProfile` / index-builder code).
   e. If the new body's digest matches the current POINTS_TO target, short-circuit.
   f. Otherwise: store the bytes in minio, MERGE the Index node, rebuild REFERENCES edges, atomically move the current POINTS_TO target to HAS_HISTORY and set the new POINTS_TO.

#### Scenario: First push creates a Slot and its POINTS_TO
- **WHEN** the first amd64 Manifest is pushed to `qnib/alpine:latest` via `generic`
- **AND** `ComputeIndexForRepoTag` runs
- **THEN** a new Slot for `(qnib/alpine, latest, generic)` SHALL be created
- **AND** a new Index with one REFERENCES edge to the amd64 Manifest SHALL be created
- **AND** the Slot's POINTS_TO SHALL point at the new Index
- **AND** no HAS_HISTORY edge SHALL exist yet

#### Scenario: Second arch extends the index
- **WHEN** an arm64 Manifest is pushed to the same `(qnib/alpine, latest, generic)` triple after the first push
- **AND** `ComputeIndexForRepoTag` runs
- **THEN** the new Index SHALL have two REFERENCES edges (amd64 + arm64)
- **AND** the previous 1-descriptor Index SHALL be archived via a HAS_HISTORY edge with the current timestamp
- **AND** the Slot's POINTS_TO SHALL point at the new 2-descriptor Index

#### Scenario: Profile with no matching rules does not get a Slot POINTS_TO
- **WHEN** the graph contains profile `cpu#zen2` whose rules do not match any pushed Manifest
- **AND** `ComputeIndexForRepoTag` runs after a push to `(qnib/alpine, latest, generic)`
- **THEN** a Slot for `(qnib/alpine, latest, cpu#zen2)` MAY or MAY NOT be created (implementation choice — both are valid per this spec)
- **AND** if it exists, it SHALL have no outgoing POINTS_TO edge

#### Scenario: Idempotent recompute does not write to the graph
- **WHEN** `ComputeIndexForRepoTag` is called twice with identical state and no new pushes in between
- **THEN** the second call SHALL detect that every Slot's new computed digest equals its current POINTS_TO target
- **AND** SHALL short-circuit before writing any HAS_HISTORY, POINTS_TO, or REFERENCES edges

### Requirement: StoreIndexUpload dual-storage for client-pushed indexes (Option α)

The `magazine.Magazine.StoreIndexUpload` method (renamed from the existing `StoreManifestListUpload`) SHALL implement strict OCI-compliant dual-storage when a client pushes an Index directly:

1. Parse and validate the index bytes.
2. Compute and verify the client-supplied digest.
3. Store the verbatim bytes in minio keyed by the client digest.
4. MERGE an `:Index` node with `origin = "client-pushed"`.
5. Ensure every member Manifest exists in the graph; return MANIFEST_BLOB_UNKNOWN otherwise.
6. MERGE REFERENCES edges from the Index to each member Manifest.
7. Resolve the pusher's Slot via `SlotGetCreate`.
8. MERGE a `HAS_PUSHED_INDEX` edge with the current timestamp.
9. Invoke `ComputeIndexForRepoTag` to recompute POINTS_TO for every profile.
10. Return the stored Index on success.

The client's original digest SHALL be pullable via `GET /v2/<repo>/manifests/<clientDigest>` under the pushing profile for as long as the `HAS_PUSHED_INDEX` edge exists. This preserves OCI `push-then-verify-by-digest` compliance required by `buildx`, `skopeo`, and `oras` clients.

#### Scenario: Client-pushed index is addressable by its own digest
- **WHEN** a client uses `skopeo copy --multi-arch all docker.io/library/alpine:3.23 dev:80/qnib/alpine-prebuilt:3.23` and the push succeeds with computed digest D
- **THEN** `HEAD /v2/qnib/alpine-prebuilt/manifests/D` under the pushing profile SHALL return HTTP 200 with `Docker-Content-Digest: D`

#### Scenario: Client-pushed index triggers per-profile recompute
- **WHEN** the above push completes
- **THEN** `ComputeIndexForRepoTag` SHALL have been invoked exactly once
- **AND** for every profile whose rules match at least one member Manifest, a Slot SHALL exist with a POINTS_TO target

#### Scenario: Client-pushed index origin is preserved
- **WHEN** the push completes
- **THEN** the created `:Index` node SHALL have `origin = "client-pushed"`
- **AND** a subsequent `ComputeIndexForRepoTag` that produces the same digest SHALL NOT overwrite `origin`
