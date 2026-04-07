## ADDED Requirements

### Requirement: ProfileGetAll database interface method
The `magazine.Database` interface SHALL expose a `ProfileGetAll(ctx context.Context) ([]types.Profile, error)` method that returns all `Profile` nodes from the Neo4j graph including their linked rules.

#### Scenario: ProfileGetAll returns all profiles with rules
- **WHEN** `ProfileGetAll` is called and the graph contains two profiles each with rules
- **THEN** the returned slice SHALL contain two `types.Profile` entries, each with a populated `Rules` slice

#### Scenario: ProfileGetAll returns empty slice when no profiles exist
- **WHEN** `ProfileGetAll` is called and no `Profile` nodes exist in the graph
- **THEN** the method SHALL return an empty slice and a nil error

#### Scenario: ProfileGetAll includes profiles with no rules
- **WHEN** a `Profile` node has no linked `Rule` nodes
- **THEN** the returned `types.Profile` entry SHALL have an empty `Rules` slice (not nil, and no error)

### Requirement: Neo4j ProfileGetAll implementation uses a single query
The `NeoBackend` SHALL implement `ProfileGetAll` using a single Cypher query that collects rules via `COLLECT()` aggregation, avoiding N+1 query patterns.

#### Scenario: Single query returns profiles and aggregated rules
- **WHEN** `ProfileGetAll` is called
- **THEN** the Cypher query SHALL use `OPTIONAL MATCH (prf)-[:USES]->(r:Rule)` with `COLLECT(r)` to return rules inline
- **THEN** no additional per-profile queries SHALL be issued

### Requirement: Profile type includes Rules field
The `types.Profile` struct SHALL include a `Rules []ProfileRule` field that is populated by `ProfileGetAll`.

#### Scenario: Profile struct carries rule data
- **WHEN** a `types.Profile` is returned by `ProfileGetAll`
- **THEN** `profile.Rules` SHALL be a slice of `types.ProfileRule` where each entry has `ID`, `Type`, `Op`, `Key`, and `Value` string fields
