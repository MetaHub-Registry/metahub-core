# profile-filtered-collections Specification

## Purpose
TBD - created by archiving change add-profile-aware-collection-queries. Update Purpose after archive.
## Requirements
### Requirement: Collection listing filtered by profile
The system SHALL provide a database query method `CollectionListingGetByRepoLikeForProfile` that returns collection listings matching a repo prefix, filtered to only collections that have at least one Slot for the given profile.

#### Scenario: Filter by existing profile with matches
- **WHEN** `CollectionListingGetByRepoLikeForProfile(ctx, "biocontainers", "generic")` is called and the profile `generic` has Slots for repos `biocontainers/bbmap` and `biocontainers/samtools` but not `biocontainers/hisat2`
- **THEN** the method SHALL return collection listings for `biocontainers/bbmap` and `biocontainers/samtools` only

#### Scenario: Filter by profile with no matching collections
- **WHEN** `CollectionListingGetByRepoLikeForProfile(ctx, "biocontainers", "cpu#strix-halo")` is called and no Slots exist for `cpu#strix-halo` under the `biocontainers` prefix
- **THEN** the method SHALL return an empty slice with no error

#### Scenario: Empty profile falls back to unfiltered
- **WHEN** `CollectionListingGetByRepoLikeForProfile(ctx, "biocontainers", "")` is called with an empty profile label
- **THEN** the method SHALL return the same results as `CollectionListingGetByRepoLike(ctx, "biocontainers")`

#### Scenario: Empty repo prefix with profile filter
- **WHEN** `CollectionListingGetByRepoLikeForProfile(ctx, "", "generic")` is called
- **THEN** the method SHALL return all collections that have at least one Slot for the `generic` profile

