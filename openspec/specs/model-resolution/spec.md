# model-resolution Specification

## Purpose
TBD - created by archiving change model-resolution-and-runtime. Update Purpose after archive.
## Requirements
### Requirement: Aggregated profile encodes VRAM capacity and hardware platform
An aggregated profile SHALL be a single label of the form `<vram>#<platform>`
(e.g. `12gb#strixhalo`) carrying both the host VRAM capacity — used to decide
which quantization fits — and the hardware platform — used as scheduling
metadata. MetaHub SHALL NOT require a separate hardware/application profile split.

#### Scenario: Profile label parsed into capacity and platform
- **WHEN** a model is pulled under the profile `12gb#strixhalo`
- **THEN** MetaHub SHALL treat 12 GB as the VRAM capacity for fit decisions
- **AND** SHALL treat `strixhalo` as the hardware-platform metadata

### Requirement: Default to the highest-precision quantization that fits
MetaHub SHALL serve the highest-precision quantization whose weights fit the
profile's VRAM capacity when a model is pulled under an aggregated profile
without a named quantization, where a larger model-layer size denotes higher
precision.

#### Scenario: No quant named resolves to the largest fitting quant
- **WHEN** a model with a `q4` quant (fits) and a `q8` quant (does not fit) is
  pulled under a profile whose VRAM capacity fits `q4` but not `q8`
- **THEN** MetaHub SHALL serve the `q4` quantization

### Requirement: Reject a named quantization that does not fit
MetaHub SHALL reject a pull whose named quantization's weights exceed the
profile's VRAM capacity, returning an OCI error (HTTP 400, code `UNSUPPORTED`)
whose message names the quantization, the profile, and the required-vs-available
capacity, and SHALL NOT serve that quantization.

#### Scenario: Infeasible named quant rejected with an OCI error
- **WHEN** a `q8` quant is requested under a profile whose VRAM capacity only fits
  `q4`
- **THEN** MetaHub SHALL respond HTTP 400 with OCI error code `UNSUPPORTED`
- **AND** the message SHALL name `q8`, the profile, and the fit failure
- **AND** SHALL NOT serve the `q8` quantization

### Requirement: User runtime parameters pass through as env vars
MetaHub SHALL let a user set model runtime parameters as plain container env vars
that the runtime reads directly: the injected config provides defaults and the
user's env vars MAY override or extend them at run time without MetaHub
involvement.

#### Scenario: User overrides a runtime parameter
- **WHEN** the injected config sets a default context size and the user sets the
  runtime's context-size env var on the container
- **THEN** the container SHALL use the user's value, with no MetaHub involvement

