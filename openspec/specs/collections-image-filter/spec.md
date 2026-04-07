## MODIFIED Requirements

### Requirement: CollectionListingGetByRepoLike excludes ORAS-only repos
The `CollectionListingGetByRepoLike` method SHALL return `CollectionListing` entries only for repos that contain at least one `ManifestData` with a container image `cfg.mediaType`. Repos that contain only ORAS artifact manifests SHALL NOT appear in the results, even if they have a matching `Collection` node in the graph.

#### Scenario: ORAS-only repo does not appear in collection listing
- **WHEN** `CollectionListingGetByRepoLike` is called with an empty repo prefix
- **AND** the graph contains a repo with only ORAS artifact manifests (`cfg.mediaType` NOT in the image allowlist)
- **THEN** the returned slice SHALL NOT contain any `CollectionListing` entry for that repo

#### Scenario: Image repo appears in collection listing
- **WHEN** `CollectionListingGetByRepoLike` is called with an empty repo prefix
- **AND** the graph contains a repo with at least one container image manifest (`cfg.mediaType` in the image allowlist)
- **THEN** the returned slice SHALL contain a `CollectionListing` entry for that repo

#### Scenario: Mixed-content repo appears in collection listing
- **WHEN** a repo contains both container image and ORAS artifact manifests
- **THEN** `CollectionListingGetByRepoLike` SHALL return a `CollectionListing` for that repo (because it has at least one image manifest)

#### Scenario: Filtering is enforced at the Collection-query level, not just ManifestData level
- **WHEN** `CollectionListingGetByRepoLike` is called
- **THEN** the Cypher query SHALL use a join or subquery on `ManifestData` with the `cfg.mediaType IN [allowlist]` predicate to exclude ORAS-only repos before returning results
- **THEN** a repo with zero qualifying manifests SHALL NOT appear in the output even with an empty `Manifests` map
