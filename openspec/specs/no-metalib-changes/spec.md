## ADDED Requirements

### Requirement: Profile propagation via gorilla context is sufficient
Metalib SHALL correctly serve the profile-scoped manifest when `req.Profile` is populated from the gorilla context `profileName` key, regardless of how that key was set by the calling API layer. No additional metalib changes are required for profile envelope token support.

#### Scenario: Profile set via gorilla context from envelope token
- **WHEN** `metahub-api` sets `profileName=zen2` in gorilla context after verifying a profile envelope token
- **THEN** `types.NewRequest` SHALL populate `req.Profile = "zen2"`
- **AND** `ManifestDataGetByCollectionTag` SHALL use `zen2` for manifest selection
- **AND** the correct profile-scoped manifest SHALL be returned to the Docker client
