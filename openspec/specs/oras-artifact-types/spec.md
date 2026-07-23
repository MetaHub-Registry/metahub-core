# oras-artifact-types Specification

## Purpose
Types representing ORAS artifacts and their layer blobs in metalib (`OrasArtifact`, `OrasListing`, `OrasBlob`).
## Requirements
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

### Requirement: OrasBlob type for layer representation

An `OrasBlob` struct SHALL represent a single layer blob with fields:
- `Digest` (string, JSON `"digest"`)
- `MediaType` (string, JSON `"media_type"`)
- `Size` (int64, JSON `"size"`)
- `Title` (string, JSON `"title"`) — extracted from `org.opencontainers.image.title` annotation
- `Annotations` (map[string]string, JSON `"annotations"`)

#### Scenario: OrasBlob carries layer fields
- **WHEN** a layer blob is represented as an `OrasBlob`
- **THEN** it carries `Digest`, `MediaType`, `Size`, `Title`, and `Annotations`
- **AND** `Title` is taken from the `org.opencontainers.image.title` annotation when present

### Requirement: OrasArtifact.Blobs uses OrasBlob type

The `OrasArtifact.Blobs` field SHALL be typed as `[]OrasBlob` instead of `[]Manifest`.

#### Scenario: Blobs field is typed as OrasBlob
- **WHEN** an `OrasArtifact` is returned
- **THEN** its `Blobs` field is a `[]OrasBlob` (not `[]Manifest`)

### Requirement: StoreManifestArtifactUpload wires Slot on push

The `StoreManifestArtifactUpload` method SHALL call `wireSlotOnPush` after linking the config blob, ensuring the ORAS artifact Manifest is reachable via the Slot-centred graph model. This is the same Slot wiring that container image pushes receive.

#### Scenario: Artifact manifest is reachable via Slot after push
- **WHEN** an ORAS artifact manifest is stored via `StoreManifestArtifactUpload`
- **AND** the request has a non-empty profile
- **THEN** the Manifest SHALL be reachable via `Repo <-[:OF_REPO]- Slot -[:HAS_PUSHED]-> Manifest`

#### Scenario: wireSlotOnPush failure is non-fatal
- **WHEN** `wireSlotOnPush` fails during artifact storage (e.g. Neo4j transient error)
- **THEN** `StoreManifestArtifactUpload` SHALL log the failure but still return success
- **AND** the Manifest and config blob link SHALL still be persisted

