# collection-profile-authority Specification

## Purpose
TBD - created by archiving change collection-profile-authority. Update Purpose after archive.
## Requirements
### Requirement: Collection manifest entries carry an explicit profile
The collection YAML schema SHALL accept an optional `profile` string on
each manifest entry. The field SHALL survive parsing into the
in-memory collection model (it is currently dropped). An empty or
absent `profile` SHALL preserve the pre-change behaviour.

#### Scenario: Profile field round-trips
- **WHEN** a collection manifest entry declares `profile: cpu#zen2`
- **THEN** the parsed manifest model SHALL carry `Profile = "cpu#zen2"`

#### Scenario: Absent profile is empty
- **WHEN** a manifest entry declares no `profile`
- **THEN** the parsed manifest's `Profile` SHALL be empty and the entry
  SHALL be materialized exactly as before this change

### Requirement: Explicit collection profile authoritatively binds the manifest
Materialization SHALL bind a manifest to the profile named in its
collection entry by injecting that profile's deterministic (`anno eq`)
rules as `HAS_ANNOTATION` annotations on the manifest, so the manifest
becomes servable under that profile.

#### Scenario: Zen 2 manifest bound and servable
- **WHEN** the zen2 manifest entry declares `profile: cpu#zen2` and is
  materialized
- **THEN** the manifest SHALL gain a `HAS_ANNOTATION` to
  `org.supercontainers.hardware.cpu.target.microarch=zen2`
- **AND** a pull under the `cpu#zen2` profile SHALL resolve that
  manifest instead of returning `MANIFEST_UNKNOWN`

### Requirement: Absent profile falls back to existing matching
A manifest entry without an explicit `profile` SHALL continue to be
matched by the existing image-annotation / generic path. The explicit
profile is authority; the image-derived annotation is the fallback used
only when no profile is declared.

#### Scenario: No-profile manifest unchanged
- **WHEN** a manifest entry has no `profile`
- **THEN** materialization SHALL NOT inject collection-profile
  annotations and the manifest's serve-time matching SHALL be identical
  to pre-change behaviour

### Requirement: Unknown collection profile fails loudly
Materialization SHALL fail with an error when a manifest entry declares
a `profile` that does not resolve to an existing profile, so a
mis-authored collection surfaces instead of silently binding nothing.

#### Scenario: Typo profile label
- **WHEN** a manifest entry declares `profile: cpu#zne2` (typo) and no
  such profile exists
- **THEN** collection materialization SHALL return an error naming the
  unresolved profile and SHALL NOT bind the manifest

