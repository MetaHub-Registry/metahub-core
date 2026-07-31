# oras-database-queries Specification

## Purpose
Graph read-queries for ORAS artifacts (repo/name/ID lookups and enrichment) over the slot-centred model.
## Requirements
### Requirement: OrasArtifactGetByRepoLike returns only genuine ORAS repos

The `OrasArtifactGetByRepoLike` method SHALL return `OrasListing` entries only for repos that contain at least one `:Manifest` with a non-image `cfg.mediaType` linked via the `[:USES]` relationship where `cfg.type = "config"`. Repos containing only container image manifests SHALL NOT appear in the results even if they share a `:Repo` name prefix.

The query SHALL walk the Slot-centred graph:

```
MATCH (c:Repo)<-[:OF_REPO]-(s:Slot)-[:HAS_PUSHED]->(md:Manifest)-[:USES]->(cfg:Blob)
WHERE cfg.type = "config"
  AND c.repo =~ $repoPattern
  AND NOT cfg.mediaType IN $containerImageConfigTypes
RETURN c.repo AS repo, cfg.mediaType AS artifactType, count(md) AS artifactCount
```

#### Scenario: Image-only repo does not appear in ORAS listing
- **WHEN** `OrasArtifactGetByRepoLike` is called with an empty repo prefix
- **AND** the graph contains a repo with only container image manifests (`cfg.mediaType` in the image allowlist)
- **THEN** the returned slice SHALL NOT contain any `OrasListing` entry for that repo

#### Scenario: ORAS-only repo appears in ORAS listing
- **WHEN** `OrasArtifactGetByRepoLike` is called with an empty repo prefix
- **AND** the graph contains a repo with at least one ORAS artifact manifest (`cfg.mediaType` NOT in the image allowlist)
- **THEN** the returned slice SHALL contain an `OrasListing` entry for that repo with the correct `artifactType` and `artifactCount`

#### Scenario: Mixed-content repo appears in ORAS listing for ORAS artifacts only
- **WHEN** a repo contains both container image and ORAS artifact manifests
- **THEN** `OrasArtifactGetByRepoLike` SHALL return the ORAS artifact entries for that repo
- **THEN** the container image manifests SHALL NOT contribute to the returned `OrasListing` entries

#### Scenario: Query uses the Slot-centred model
- **WHEN** `OrasArtifactGetByRepoLike` runs
- **THEN** the Cypher query SHALL match on `:Repo`, `:Slot`, `:Manifest`, and `:Blob` labels
- **AND** SHALL traverse `Repo <-[:OF_REPO]- Slot -[:HAS_PUSHED]-> Manifest -[:USES]-> Blob`
- **AND** SHALL NOT reference `:Collection`, `:ManifestData`, the legacy `:Manifest` wrapper label, or the `[:UPSTREAM]` relationship

### Requirement: OrasArtifactEnrich populates layers and annotations

The `OrasArtifactEnrich` function SHALL populate the `Blobs` and `Annotations` fields of an `OrasArtifact` by querying existing graph data. No graph writes occur.

#### Scenario: Manifest annotations are fetched
- **WHEN** `OrasArtifactEnrich` is called with a valid manifest ID
- **AND** the manifest has `HAS_ANNOTATION` edges to Annotation nodes
- **THEN** `artifact.Annotations` SHALL contain all key/value pairs from those Annotation nodes

#### Scenario: Layer blobs with annotations are fetched
- **WHEN** `OrasArtifactEnrich` is called with a valid manifest ID
- **AND** the manifest has `HAS_LAYER` edges to Blob nodes with `type = "layer"`
- **AND** those Blob nodes have `HAS_ANNOTATION` edges to Annotation nodes
- **THEN** `artifact.Blobs` SHALL contain one `OrasBlob` per layer, ordered by `r.order`
- **AND** each `OrasBlob` SHALL have `Digest`, `MediaType`, `Size`, and `Annotations` populated
- **AND** if an annotation has `key = "org.opencontainers.image.title"`, the `OrasBlob.Title` field SHALL be set to its value

#### Scenario: Layer without annotations
- **WHEN** a layer Blob has no `HAS_ANNOTATION` edges
- **THEN** the `OrasBlob.Annotations` SHALL be an empty map and `Title` SHALL be empty string

#### Scenario: Manifest without layers
- **WHEN** the manifest has no `HAS_LAYER` edges
- **THEN** `artifact.Blobs` SHALL be an empty slice (not nil)

### Requirement: OrasArtifactGetByName returns enriched artifact

The `OrasArtifactGetByName` method SHALL call `OrasArtifactEnrich` after constructing the base artifact, so the returned `OrasArtifact` includes layers and annotations.

#### Scenario: Fetch by name includes layers and annotations
- **WHEN** `OrasArtifactGetByName` is called for an existing artifact
- **THEN** the returned `OrasArtifact` has `Blobs` and `Annotations` populated via `OrasArtifactEnrich`

### Requirement: OrasArtifactGetByID returns enriched artifact

The `OrasArtifactGetByID` method SHALL call `OrasArtifactEnrich` after constructing the base artifact, so the returned `OrasArtifact` includes layers and annotations.

#### Scenario: Fetch by ID includes layers and annotations
- **WHEN** `OrasArtifactGetByID` is called with a valid manifest ID
- **THEN** the returned `OrasArtifact` has `Blobs` and `Annotations` populated via `OrasArtifactEnrich`

### Requirement: OrasArtifactGetByName uses Slot-centred model

The `OrasArtifactGetByName` method SHALL query for a single ORAS artifact by repo name and artifact type using the Slot-centred graph path: `Repo <-[:OF_REPO]- Slot -[:HAS_PUSHED]-> Manifest -[:USES]-> Blob`.

#### Scenario: Artifact found by repo and type
- **WHEN** `OrasArtifactGetByName` is called with a repo name and artifact type
- **AND** the graph contains a Slot-connected Manifest with a matching config blob mediaType
- **THEN** the method SHALL return an `OrasArtifact` with the correct digest, repo, and artifact type

#### Scenario: Artifact not found
- **WHEN** `OrasArtifactGetByName` is called with a repo name and artifact type that has no matching Slot-connected Manifest
- **THEN** the method SHALL return an error

### Requirement: OrasArtifactGetByID uses Slot-centred model

The `OrasArtifactGetByID` method SHALL query for a single ORAS artifact by Neo4j node ID using the Slot-centred graph path: `Repo <-[:OF_REPO]- Slot -[:HAS_PUSHED]-> Manifest -[:USES]-> Blob`.

#### Scenario: Artifact found by ID
- **WHEN** `OrasArtifactGetByID` is called with a valid Manifest node ID
- **AND** the Manifest is connected via a Slot with a non-image config blob
- **THEN** the method SHALL return an `OrasArtifact` with the correct digest, repo, and artifact type

#### Scenario: Artifact not found by ID
- **WHEN** `OrasArtifactGetByID` is called with a Manifest node ID that has no Slot connection or is a container image
- **THEN** the method SHALL return an error

### Requirement: ORAS artifact push creates Slot graph structure

When `StoreManifestArtifactUpload` processes an ORAS artifact manifest, it SHALL call `wireSlotOnPush` to create the Slot node and its relationships, producing the graph:

```
Repo <-[:OF_REPO]- Slot -[:OF_TAG]-> Tag
                    |
                    +-[:OF_PROFILE]-> Profile
                    +-[:HAS_PUSHED]-> Manifest
```

#### Scenario: ORAS push creates Slot with correct relationships
- **WHEN** an ORAS artifact manifest is pushed with repo `hello-artifact`, tag `v1`, and profile `generic`
- **THEN** a Slot node SHALL be created with `repo = "hello-artifact"`, `tag = "v1"`, `profile = "generic"`
- **AND** the Slot SHALL have `[:OF_REPO]` to the Repo node, `[:OF_TAG]` to the Tag node, `[:OF_PROFILE]` to the Profile node, and `[:HAS_PUSHED]` to the Manifest node

#### Scenario: ORAS push with empty profile skips Slot creation
- **WHEN** an ORAS artifact manifest is pushed with an empty profile
- **THEN** `wireSlotOnPush` SHALL return without creating a Slot (defensive guard)

### Requirement: ManifestLinkToArtifactConfig sets config blob type

The `ManifestLinkToArtifactConfig` method SHALL set `b.type = "config"` on the config blob node in addition to setting `b.mediaType`. This ensures ORAS read queries that filter on `cfg.type = "config"` can find artifact config blobs.

#### Scenario: Config blob has type property after artifact link
- **WHEN** `ManifestLinkToArtifactConfig` links a Manifest to its config Blob
- **THEN** the Blob node SHALL have `type = "config"` and `mediaType` set to the config descriptor's media type

