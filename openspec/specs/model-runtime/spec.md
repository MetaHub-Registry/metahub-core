# model-runtime Specification

## Purpose
TBD - created by archiving change model-resolution-and-runtime. Update Purpose after archive.
## Requirements
### Requirement: A runtime is a registered application-container artefact
A runtime SHALL be a registered application-container artefact carrying coarse
compatibility metadata `{formats, config-contract}` — the model formats it can run
(e.g. `gguf`) and the config contract it honors (e.g. `qnib.model.config.v1`, the
`QNIB_MODEL_CONFIG_DIR` env contract) — so MetaHub can decide which runtimes can
serve a given model and config.

#### Scenario: Runtime declares compatibility
- **WHEN** a runtime is registered with formats `[gguf]` and config-contract
  `qnib.model.config.v1`
- **THEN** MetaHub SHALL record the runtime and its compatibility for composition
  matching

### Requirement: A composition binds a runtime to a resolved model and config
MetaHub SHALL assemble a composition as a compatible runtime plus the resolved
model-layer quantization and the resolved config, where compatible MEANS the
model's format is in the runtime's formats and the config's contract matches the
runtime's config-contract (an empty runtime config-contract matches any).

#### Scenario: Composition assembled from a compatible runtime
- **WHEN** a model has a `gguf` quant and a `qnib.model.config.v1` config, and a
  runtime declares formats `[gguf]` and config-contract `qnib.model.config.v1`
- **THEN** MetaHub SHALL offer a composition binding that runtime to the resolved
  quant and config

#### Scenario: Incompatible runtime excluded
- **WHEN** a model is `gguf` and a runtime declares formats `[safetensors]`
- **THEN** MetaHub SHALL NOT offer a composition with that runtime

### Requirement: Runtimes and compositions are served over the API
MetaHub SHALL expose a model's compatible runtimes and its resolvable compositions
over the API so a client (the model card) can present a runnable bundle without
bespoke graph traversal.

#### Scenario: Card fetches compositions
- **WHEN** a client requests a model's compositions
- **THEN** MetaHub SHALL return each composition's runtime, quantization, and
  config

