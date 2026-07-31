# manipulation-apply-select Specification

## Purpose
TBD - created by archiving change add-manipulayer-select. Update Purpose after archive.
## Requirements
### Requirement: ManipuLayer layer selection via slice syntax
The manipulation schema SHALL accept an optional `select` string on
each `ManipuLayer` entry. Supported values are Python-style slice
expressions: `[N]` (single index), `[N:]`, `[:M]`, `[N:M]`,
`[N:M:S]` (step), with negative indices relative to the source
image's layer count. Missing or empty `select` SHALL be treated as
`[:]` (all layers).

#### Scenario: Skip first layer
- **WHEN** a ManipuLayer has `select: "[1:]"` and the source image
  has three layers `[A, B, C]`
- **THEN** the layers appended to the manipulated manifest SHALL be
  `[B, C]` and the manipulated config's `RootFS.DiffIDs` SHALL omit
  the diffID corresponding to `A`

#### Scenario: Single index with negative form
- **WHEN** `select: "[-1]"` is applied to a three-layer source
- **THEN** the appended slice SHALL be `[C]` only

#### Scenario: Absent select is identity
- **WHEN** a ManipuLayer has no `select` field
- **THEN** every layer from the source image SHALL be appended,
  matching the pre-change behaviour

### Requirement: Out-of-range select fails the pull
The apply step SHALL refuse to produce a manipulated manifest when
the resolved slice references an index or bound outside the actual
layer list. This applies with STRICT (non-Python) semantics: a
single index, a range start, or a range end that falls outside
`[0, M]` after negative-wrapping SHALL fail the pull rather than
silently clamping. The failure SHALL surface as a 5xx with the OCI
error envelope carrying an `"out of range for input of length M"`
message.

#### Scenario: Single index out of range
- **WHEN** a ManipuLayer has `select: "[5]"` and the source image
  has three layers
- **THEN** the pull SHALL return an error and SHALL NOT persist a
  partial manipulated manifest

#### Scenario: Range bound out of range
- **WHEN** a ManipuLayer has `select: "[2:999]"` or `select: "[6:]"`
  and the source image has three layers
- **THEN** the pull SHALL return an error and SHALL NOT silently
  clamp the bound to the layer count

### Requirement: Empty selection fails the pull
The apply step SHALL refuse to produce a manipulated manifest when a
range select resolves to zero layers (e.g. `[0:0]`, `[3:1]`, or
`[3:]` on a three-layer source). Appending no layers is treated as a
misconfiguration. The failure SHALL surface as a 5xx with the OCI
error envelope carrying an `"empty layer set"` message.

#### Scenario: Explicitly empty range
- **WHEN** a ManipuLayer has `select: "[0:0]"`
- **THEN** the pull SHALL return an error and SHALL NOT persist a
  manipulated manifest with the overlay's layers omitted

#### Scenario: Start at layer count
- **WHEN** `select: "[3:]"` is applied to a three-layer source
- **THEN** the pull SHALL return an error (empty layer set) rather
  than appending nothing silently

### Requirement: Layer descriptors and DiffIDs stay in lockstep
The apply step SHALL apply the same slice to the OCI config's
`RootFS.DiffIDs` that it applies to the layer descriptor list, so
the number and order of layers and diffIDs remain equal.

#### Scenario: DiffID mirror
- **WHEN** `select: "[1:]"` is applied to a source with layer list
  `[A, B, C]` and diffIDs `[da, db, dc]`
- **THEN** the manipulated OCI config SHALL carry diffIDs
  `[origDiffIDs..., db, dc]` in the same order as the layer
  descriptors

### Requirement: Invalid syntax rejected at parse time
The parser SHALL reject malformed slice strings with a clear error
at manipulation-parse time so broken YAML files never produce
graph nodes.

#### Scenario: Negative step rejected
- **WHEN** a YAML file contains `select: "[::-1]"`
- **THEN** the parser SHALL return an error naming the offending
  field and the manipulation SHALL NOT be created

#### Scenario: Too many colons rejected
- **WHEN** `select: "[1:2:3:4]"`
- **THEN** the parser SHALL return an error

