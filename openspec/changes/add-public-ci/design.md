# Design — public CI for metahub-core

## Stack and boundary

Go 1.26.0 declared in `go.mod` (CI pins `1.26.x`). The old `go 1.19` floor was
raised by the dependency refresh.

Fast checks:

- `go build ./...`
- `go vet ./...`
- `golangci-lint` with the committed `.golangci.yml`
- `go test -short ./...` for container-free packages
- `gitleaks` with the committed `.gitleaks.toml`
- DCO trailer check
- `go-licenses` allowlist and generated `THIRD_PARTY_NOTICES.md`
- `govulncheck` (advisory while the dependency baseline is cleared)
- outbound-host allowlist

Container-backed checks: `neo`, `minio`, `magazine` and `tracing` start containers; their `TestMain`/tests exit early under `-short`. `sqlite` uses a temp file.

The integration lane runs `go test -p 1` so only one Neo4j/MinIO/Jaeger container set is alive at a time.

## Decisions

### Self-contained, fork-safe workflows

The workflows live in this repository. A cross-repository reusable workflow
would need either a public `.github` repository before the launch gate or
private access that a fork does not receive. Every PR-triggered workflow is
therefore self-contained, declares `permissions: contents: read`, pins
third-party actions by SHA and references no secret.

### One aggregator

`fast` depends on every fast job and fails when any of them fails or is
cancelled. Marking only `fast` as a required check at the visibility flip keeps
the branch-protection configuration stable as checks are added.

### Container-backed checks gate releases

Tests that need Docker run in `integration.yml` on release tags and
`workflow_dispatch`, with no secrets. They never run on a pull request.

### Dependencies and licences

The licence check fails on a dependency outside the allowlist and the notices
file is regenerated in CI; a drift fails the job. `govulncheck` is advisory
(`continue-on-error: true`) until the remaining baseline is cleared. The
2026-10-30 scan reported 42 reachable findings: 30 in the Go standard library
(fixed by `go1.26.9`/`go1.26.10`, which `1.26.x` picks up in CI) and 12 through
modules — `containerd v1.7.33` (7), `docker v25.0.6` (3) and `regclient v0.4.5`
(2). Six of the module findings have no patched version. The job still runs on
every pull request.

### Outbound hosts

Non-test source may reference only hosts on `.github/outbound-allowlist.txt`.
A new host fails the job and must be added deliberately in review. This is a
review gate, not a network sandbox.

## Risks

| Risk | Mitigation |
|---|---|
| A workflow needs a secret when run from a fork | it belongs in the integration lane or is dropped |
| Action supply chain | every action is pinned to a full commit SHA |
| Dependency advisories | Dependabot opens weekly PRs; the Go vulnerability job is advisory until the current baseline is cleared |
