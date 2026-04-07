## ADDED Requirements

### Requirement: OrasArtifact type
The system SHALL provide an `OrasArtifact` Go struct in `pkg/types/` representing a single ORAS artifact manifest with all metadata needed for API responses.

#### Scenario: OrasArtifact has required fields
- **WHEN** an `OrasArtifact` value is populated from a Neo4j query result
- **THEN** it SHALL expose: `ID` (int64, Neo4j node ID), `Repo` (string), `Digest` (string), `MediaType` (string), `ArtifactType` (string), `Annotations` (map[string]string), `Subject` (*ManifestData, nullable), `Blobs` ([]ManifestData)

#### Scenario: OrasArtifact with no subject
- **WHEN** an ORAS artifact manifest has no `subject` field in the registry
- **THEN** `OrasArtifact.Subject` SHALL be `nil`

#### Scenario: OrasArtifact with blobs
- **WHEN** an ORAS artifact manifest references one or more blob descriptors
- **THEN** `OrasArtifact.Blobs` SHALL contain one `ManifestData` entry per blob, with at minimum `Digest`, `MediaType`, and `Size` populated

### Requirement: OrasListing type
The system SHALL provide an `OrasListing` Go struct in `pkg/types/` used for repo-tree navigation of ORAS artifacts, parallel to `CollectionListing`.

#### Scenario: OrasListing has navigation fields
- **WHEN** an `OrasListing` value is populated from a database query
- **THEN** it SHALL expose: `Repo` (string), `ArtifactType` (string), `ArtifactCount` (int64)

#### Scenario: OrasListing used for tree navigation
- **WHEN** the ORAS list endpoint traverses a repo prefix
- **THEN** multiple `OrasListing` values MAY share the same `Repo` prefix but differ in `ArtifactType`
