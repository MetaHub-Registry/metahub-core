# profiles-graph-query Specification

## Purpose
Graph read-queries for profiles and their hardware, rules, and pins.

## Requirements

### Requirement: ProfileGetAll database interface method

`ProfileGetAll` returns all `:Profile` nodes including their linked rules. The internal cypher walk SHALL reference the new node labels where applicable (`:Manifest` instead of `:ManifestData` if the implementation walks back from the rule-shortcut side), but the public signature and return type are stable.

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

The single-query `COLLECT` aggregation pattern SHALL be preserved. No functional change from the refactor — this requirement is carried over unchanged but listed here so the spec reflects the post-refactor state explicitly.

#### Scenario: Single query returns profiles and aggregated rules
- **WHEN** `ProfileGetAll` is called
- **THEN** the Cypher query SHALL use `OPTIONAL MATCH (prf)-[:USES]->(r:Rule)` with `COLLECT(r)` to return rules inline
- **THEN** no additional per-profile queries SHALL be issued

### Requirement: Profile type includes Rules field

The `types.Profile` struct SHALL include a `Rules []ProfileRule` field that is populated by `ProfileGetAll`. This requirement is carried through the refactor unchanged — the struct shape is not affected by the graph model rename.

#### Scenario: Profile struct carries rule data
- **WHEN** a `types.Profile` is returned by `ProfileGetAll`
- **THEN** `profile.Rules` SHALL be a slice of `types.ProfileRule` where each entry has `ID`, `Type`, `Op`, `Key`, and `Value` string fields

### Requirement: ProfileGetAllForSlotWalk helper returns profiles keyed for slot resolution

The profile-query capability SHALL expose a helper suitable for the Slot-centred compute path: `ProfileGetAllWithRuleExpressions(ctx) ([]types.Profile, error)` (or equivalent) that returns all profiles with their rule expressions already parsed (so the compute path doesn't re-parse the expression string on every invocation).

#### Scenario: Helper returns profiles with parsed rule expressions
- **WHEN** `ProfileGetAllWithRuleExpressions` is called on a graph with two profiles, one with `"os:linux=true && cpu:noop=true"` and one with `"cpu:zen2=true"`
- **THEN** the returned slice SHALL contain two profiles, each with a parsed expression AST already attached

#### Scenario: Helper is idempotent and read-only
- **WHEN** `ProfileGetAllWithRuleExpressions` is called twice in succession
- **THEN** both calls SHALL return equivalent results
- **AND** neither call SHALL mutate the graph
