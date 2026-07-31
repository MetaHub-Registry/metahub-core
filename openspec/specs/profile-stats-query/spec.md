# profile-stats-query Specification

## Purpose
TBD - created by archiving change add-profile-aware-collection-queries. Update Purpose after archive.
## Requirements
### Requirement: Profile detail with usage statistics
The system SHALL provide a database query method `ProfileGetByLabelWithStats` that returns a profile by its label together with the count of distinct collections (Slots) and distinct manifests served under it.

#### Scenario: Profile with Slots and manifests
- **WHEN** `ProfileGetByLabelWithStats(ctx, "cpu#zen3")` is called and the profile has 5 Slots pointing to Indexes referencing 12 manifests
- **THEN** the method SHALL return a `ProfileStats` with `CollectionCount == 5` and `ManifestCount == 12`

#### Scenario: Profile with no Slots
- **WHEN** `ProfileGetByLabelWithStats(ctx, "cpu#skylake")` is called and the profile exists but has no Slots
- **THEN** the method SHALL return a `ProfileStats` with `CollectionCount == 0` and `ManifestCount == 0`

#### Scenario: Unknown profile label
- **WHEN** `ProfileGetByLabelWithStats(ctx, "nonexistent")` is called and no profile with that label exists
- **THEN** the method SHALL return `found == false` with no error

### Requirement: Manifest profile label resolution
The system SHALL provide a database query method `ManifestGetProfileLabels` that, given a list of manifest digests, returns a map of digest to profile label list.

#### Scenario: Manifests served under multiple profiles
- **WHEN** `ManifestGetProfileLabels(ctx, ["sha256:abc", "sha256:def"])` is called and manifest `sha256:abc` is referenced by Indexes in Slots for profiles `generic` and `cpu#zen3`, while `sha256:def` is only in `generic`
- **THEN** the method SHALL return `{"sha256:abc": ["generic", "cpu#zen3"], "sha256:def": ["generic"]}`

#### Scenario: Manifest not served under any profile
- **WHEN** a digest is provided that has no Slot→Index→Manifest path
- **THEN** the digest SHALL be absent from the returned map (or mapped to an empty slice)

#### Scenario: Empty digest list
- **WHEN** `ManifestGetProfileLabels(ctx, [])` is called
- **THEN** the method SHALL return an empty map with no error

