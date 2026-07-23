# manipulation-graph-extended Specification

## Purpose
TBD - created by archiving change extend-manipulation-schema. Update Purpose after archive.
## Requirements
### Requirement: ManipuConfig nodes persist default and variant manipulation blocks
The Neo4j backend SHALL persist extended manipulation data as `ManipuConfig` nodes
linked from the owning `Manipulation` via `HAS_DEFAULT` (at most one per
manipulation) and `HAS_VARIANT` (zero or more, each carrying a `rule` property on
the edge).

#### Scenario: Defaults-only manipulation stores one config node
- **WHEN** `ManipulationCreate` is called with a `Manipulation` whose `Default` is
  non-empty and `Variants` is empty
- **THEN** the graph SHALL contain exactly one `ManipuConfig` linked via
  `HAS_DEFAULT` from the `Manipulation` node

#### Scenario: Variants-only manipulation stores one config node per variant
- **WHEN** `ManipulationCreate` is called with a `Manipulation` whose `Variants`
  slice has two entries
- **THEN** the graph SHALL contain two `ManipuConfig` nodes, each linked via
  `HAS_VARIANT {rule}` with the correct rule string on the edge

#### Scenario: Default and variants coexist
- **WHEN** a manipulation has both `Default` and `Variants`
- **THEN** the graph SHALL contain one `ManipuConfig` linked via `HAS_DEFAULT`
  plus N linked via `HAS_VARIANT`

### Requirement: ManipuCommand nodes carry entrypoint and cmd operations
The backend SHALL persist `ManipuCommand` nodes for entrypoint and cmd with
properties `op` (one of `replace`, `prepend`, `append`) and `value` (list of
string). Each `ManipuConfig` SHALL link to at most one entrypoint command
(`HAS_ENTRYPOINT`) and at most one cmd command (`HAS_CMD`).

#### Scenario: Replace entrypoint persists
- **WHEN** a manipulation config has an entrypoint with `op=replace, value=[...]`
- **THEN** the graph SHALL contain a `ManipuCommand` linked via `HAS_ENTRYPOINT`
  with those properties

### Requirement: Layers attach to ManipuConfig preserving order
The backend SHALL link layers to their owning `ManipuConfig` via
`HAS_LAYER {order}` where `order` is the zero-based index of the layer within that
config's `Layers` slice.

#### Scenario: Two layers keep ordering on read
- **WHEN** a `ManipuConfig` has two `Layers` inserted in order (A, B) and
  `ManipulationFromResult` is called
- **THEN** the returned config's `Layers` slice SHALL be `[A, B]`

### Requirement: ManipulationsWithConfigByProfileID returns hydrated tree
The `Database` interface SHALL expose
`ManipulationsWithConfigByProfileID(ctx, prfID) ([]Manipulation, error)` that
returns every `Manipulation` linked to the given `Profile` via `[:USES]` with
`Default`, `Variants`, and all `ManipuLayer`/`ManipuCommand` leaves hydrated.

#### Scenario: Profile with two manipulations
- **WHEN** a profile has `[:USES]` relationships to two manipulations, one legacy
  (flat layers) and one with the new default+variants model
- **THEN** the method SHALL return both manipulations with the appropriate fields
  populated (legacy manipulation exposes `ManipuLayers`; new one exposes `Default`
  and `Variants`)

#### Scenario: Profile with no manipulations
- **WHEN** a profile has no linked manipulations
- **THEN** the method SHALL return an empty slice with no error

#### Scenario: Unknown profile id
- **WHEN** the given profile id does not exist
- **THEN** the method SHALL return an empty slice with no error

