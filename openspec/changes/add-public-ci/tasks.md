# Tasks — public CI for metahub-core

## 1. Fast lane

- [x] 1.1 Add `.github/workflows/ci.yml` with `push`/`pull_request` triggers,
      `permissions: contents: read`, concurrency cancellation and a `fast`
      aggregator.
- [x] 1.2 Add the build, static-analysis, lint and container-free test jobs.
- [x] 1.3 Add gitleaks (pinned CLI, committed config), DCO and the outbound-host
      allowlist. Add the licence allowlist and generated `THIRD_PARTY_NOTICES.md`.
- [x] 1.4 Add `.github/dependabot.yml`.

## 2. Integration lane

- [x] 2.1 Add `integration.yml` for the full Go suite with `-p 1` and Docker
      available.

## 3. Follow-ups (open-sourcing gate)

- [ ] 3.1 Clear the `govulncheck` baseline and make the job blocking.
      Scan on 2026-10-30: **42 reachable findings** — 30 stdlib (fixed by
      `go1.26.9`/`go1.26.10`; CI's `1.26.x` should pick that up, local
      `go1.26.0` does not) and 12 module findings:
      `containerd v1.7.33` (7; 2 close at `1.7.35`, 1 at `1.7.36`, 4 no fix),
      `docker v25.0.6` (3; 1 closes at `25.0.13`, 2 no fix),
      `regclient v0.4.5` (2; close at `0.7.1`/`0.11.5`).
      Remaining: bump the three modules, re-scan on the CI Go patch, and record
      dated acceptances in `CONTEXT.md` for the 6 findings with no fix. The
      Neo4j 5 lane touches the same module graph.
- [ ] 3.2 Run the fast lane from a fork pull request before the visibility flip
      and record the run URL here. This repository has no private-module
      dependency, so every fast job already runs on a fork; the check is the
      proof, not a code change.
