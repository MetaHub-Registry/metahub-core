## MODIFIED Requirements

### Requirement: OrasArtifactGetByRepoLike returns only genuine ORAS repos
The `OrasArtifactGetByRepoLike` method SHALL return `OrasListing` entries only for repos that contain at least one `ManifestData` with a non-image `cfg.mediaType` linked via the `CREATED` relationship. Repos that contain only container image manifests SHALL NOT appear in the results even if they share a `Collection` node prefix.

#### Scenario: Image-only repo does not appear in ORAS listing
- **WHEN** `OrasArtifactGetByRepoLike` is called with an empty repo prefix
- **AND** the graph contains a repo with only container image manifests (`cfg.mediaType` in the image allowlist)
- **THEN** the returned slice SHALL NOT contain any `OrasListing` entry for that repo

#### Scenario: ORAS-only repo appears in ORAS listing
- **WHEN** `OrasArtifactGetByRepoLike` is called with an empty repo prefix
- **AND** the graph contains a repo with at least one ORAS artifact manifest (`cfg.mediaType` NOT in the image allowlist, `CREATED` relationship)
- **THEN** the returned slice SHALL contain an `OrasListing` entry for that repo with the correct `artifactType` and `artifactCount`

#### Scenario: Mixed-content repo appears in ORAS listing for ORAS artifacts only
- **WHEN** a repo contains both container image and ORAS artifact manifests
- **THEN** `OrasArtifactGetByRepoLike` SHALL return the ORAS artifact entries for that repo
- **THEN** the container image manifests SHALL NOT contribute to the returned `OrasListing` entries
