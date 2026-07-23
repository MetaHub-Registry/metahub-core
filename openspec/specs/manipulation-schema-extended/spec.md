# manipulation-schema-extended Specification

## Purpose
TBD - created by archiving change extend-manipulation-schema. Update Purpose after archive.
## Requirements
### Requirement: Extended manipulation YAML schema with defaults and variants
The YAML parser SHALL accept a manipulation file with an optional top-level defaults
block (`env`, `workdir`, `user`, `volumes`, `entrypoint`, `cmd`, `layers`) and an
optional `variants` map keyed by `expr` rule strings, where each variant value is a
defaults-shaped block that overrides or extends the top-level fields when its rule
matches.

#### Scenario: Full new-form manipulation parses
- **WHEN** a YAML file defines top-level `env`, `workdir`, `user`, `volumes`,
  `entrypoint`, `cmd`, `layers` and two entries under `variants`
- **THEN** the parser SHALL return a `Manipulation` whose `Default` carries the
  top-level fields and whose `Variants` slice contains one entry per rule key with
  the variant-specific fields populated

#### Scenario: Variant-only manipulation parses
- **WHEN** a YAML file defines only the `variants` map with no top-level defaults
- **THEN** the parser SHALL return a `Manipulation` with empty `Default` and
  populated `Variants`

#### Scenario: Defaults-only manipulation parses
- **WHEN** a YAML file defines only top-level fields with no `variants`
- **THEN** the parser SHALL return a `Manipulation` with `Default` populated and an
  empty `Variants` slice

### Requirement: Command operation type for CMD and ENTRYPOINT
The system SHALL provide a `ManipuCommand` type with an `Op` field whose value is
one of `replace`, `prepend`, or `append`, and a `Value` field carrying the
string-slice payload. The parser SHALL reject any other value of `Op`.

#### Scenario: Valid op values accepted
- **WHEN** `entrypoint.op` is `replace`, `prepend`, or `append`
- **THEN** the parser SHALL accept the value

#### Scenario: Invalid op rejected
- **WHEN** `cmd.op` is `overwrite` or any other unknown string
- **THEN** the parser SHALL return a validation error naming the offending field

### Requirement: Legacy flat layers form still accepted
The YAML parser SHALL continue to accept the legacy flat `layers: [{image, rule}]`
form when no new top-level defaults and no `variants` are present, rewriting each
legacy layer as a one-layer variant keyed by the layer's `rule` string, and SHALL
emit a deprecation warning log line referencing the YAML file path.

#### Scenario: Legacy file parses with deprecation warning
- **WHEN** a YAML file uses the old `layers: [{image, rule}]` shape and no new
  fields
- **THEN** the parser SHALL return a `Manipulation` whose `Variants` slice has one
  entry per legacy layer (keyed by that layer's `rule`), and SHALL log a
  deprecation warning that includes the file path

### Requirement: Mixed-form files rejected
The parser SHALL reject any YAML file that sets both a per-layer `rule:` on a
top-level `layers` entry AND any of: top-level defaults fields (`env`, `workdir`,
`user`, `volumes`, `entrypoint`, `cmd`) or a `variants` map.

#### Scenario: Mixed form rejected
- **WHEN** a YAML file has both per-layer `rule:` entries AND a `variants:` block
- **THEN** the parser SHALL return a non-nil error whose message names the
  ambiguity and does NOT write anything to the graph

### Requirement: Variant keys must be parseable rule strings
The parser SHALL validate each `variants` map key via `utils.ExprGetLabels` and
SHALL reject keys that cannot be parsed.

#### Scenario: Invalid rule key rejected
- **WHEN** a YAML file has a `variants` key such as `arch&arm64`
- **THEN** the parser SHALL return an error whose message includes the offending
  key

