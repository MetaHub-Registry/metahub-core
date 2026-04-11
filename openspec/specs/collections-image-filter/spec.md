## Requirements

### Requirement: RepoListingGetByRepoLike excludes ORAS-only repos

This capability uses `RepoListing` semantics over `:Repo` nodes. The filter predicate is: listings SHALL include only repos that contain at least one `:Manifest` with a container image `cfg.mediaType`. Repos that contain only ORAS artifact manifests SHALL NOT appear in the results.

Method:

- `RepoListingGetByRepoLike`
- Return type `[]RepoListing`

The underlying walk SHALL use the `:Repo` label, and SHALL reach manifests via `(Slot)-[:HAS_PUSHED]->(:Manifest)` or `(Manifest)-[:ORIGINAL]->(Upstream {repo})`.

#### Scenario: ORAS-only repo does not appear in repo listing
- **WHEN** `RepoListingGetByRepoLike` is called with an empty repo prefix
- **AND** the graph contains a repo with only ORAS artifact manifests (`cfg.mediaType` NOT in the image allowlist)
- **THEN** the returned slice SHALL NOT contain any `RepoListing` entry for that repo

#### Scenario: Image repo appears in repo listing
- **WHEN** `RepoListingGetByRepoLike` is called with an empty repo prefix
- **AND** the graph contains a repo with at least one container image `:Manifest` (`cfg.mediaType` in the image allowlist)
- **THEN** the returned slice SHALL contain a `RepoListing` entry for that repo

#### Scenario: Mixed-content repo appears in repo listing
- **WHEN** a repo contains both container image and ORAS artifact manifests
- **THEN** `RepoListingGetByRepoLike` SHALL return a `RepoListing` for that repo (because it has at least one image manifest)

#### Scenario: Filtering uses the new label names
- **WHEN** `RepoListingGetByRepoLike` is called
- **THEN** the Cypher query SHALL match on `:Repo`, `:Slot`, and `:Manifest` labels
- **AND** SHALL NOT reference `:Collection`, `:ManifestData`, or the legacy `:Manifest` wrapper label
