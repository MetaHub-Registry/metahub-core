## ADDED Requirements

### Requirement: Core lookups accept natural keys

The core SHALL expose lookups by manifest `digest`, profile `label`, and
`repo` + `digest` for ORAS artifacts. An exported lookup reached from the
public API SHALL NOT take a Neo4j element ID. Inside the backend, an element ID
MAY be used after the natural key has been resolved, including in `id(n)`
joins.

#### Scenario: Layers by digest

- **WHEN** a caller asks for the layers of a manifest
- **THEN** it passes the manifest `digest`
- **AND** the returned layers are identified by their digests

#### Scenario: Manipulations by label

- **WHEN** a caller asks for a profile's manipulations
- **THEN** it passes the profile `label`
- **AND** each manipulation carries its `label`

#### Scenario: ORAS by digest

- **WHEN** a caller asks for an ORAS artifact
- **THEN** it passes `repo` + `digest`
- **AND** the tree navigation may use `repo` + `artifact_type`

### Requirement: Serialised core types hide storage IDs

The core types that are serialised into public payloads SHALL NOT emit a
Neo4j element ID or a SQLite rowid. `OrasArtifact`, `Manipulation`,
`ManipuConfig` and `ManipuCommand` SHALL tag `ID` with `json:"-"`;
`www.User.ID` and `www.Secret.ID` SHALL do the same.

#### Scenario: Artifact JSON

- **WHEN** an `OrasArtifact` is marshalled
- **THEN** the JSON carries `repo` and `digest`
- **AND** no `id` key

#### Scenario: User and secret JSON

- **WHEN** a `www.User` or `www.Secret` is marshalled
- **THEN** the JSON carries `username` / `name`
- **AND** no `id` key

### Requirement: The change is documented

`CHANGELOG.md` SHALL record the hidden IDs and the new lookup methods as a
breaking pre-1.0 change.

#### Scenario: Release note

- **WHEN** a user reads the changelog for the release that contains this change
- **THEN** the hidden fields and the new methods are listed
