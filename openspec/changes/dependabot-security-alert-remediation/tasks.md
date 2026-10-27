# Tasks — remediate the Dependabot backlog

Repository: `metahub-core`. Session provenance: `open-source-metahub`,
2026-10-27. `metahub-core` is checked out as `metalib/` in that session.

## 1. Bumps

- [x] 1.1 `github.com/containerd/containerd` -> `1.7.33` (closes 2 findings: high; 1.7.29 was the lower floor)
- [x] 1.2 `github.com/containers/image/v5` -> `5.29.3` (closes 1 finding: high)
- [x] 1.3 `github.com/docker/distribution` -> `2.8.3+incompatible` (advisory floor is `2.8.2-beta.1`; `containers/image v5.29.3` requires `2.8.3`, which satisfies it)
- [x] 1.4 `github.com/docker/docker` -> `25.0.6+incompatible` (closes the critical GHSA-v23v-6jw2-98fq; the two `<= 28.5.2` highs have no fix — accepted in `CONTEXT.md`)
- [x] 1.5 `github.com/klauspost/compress` -> `1.18.7` (closes 1 finding: high)

## 2. No-fix advisories

- [x] 2.1 Record a dated acceptance in `CONTEXT.md` for each advisory
      whose `first_patched_version` is null (advisory id, reachability
      argument, revisit date). Done for GHSA-rg2x-37c3-w2rh and
      GHSA-x86f-5xw2-fm2r (both daemon-side; client-only usage).

## 3. Verification

- [x] 3.1 `GOWORK=off go build ./... && go vet ./...`
- [ ] 3.2 `./test.sh 2>&1 > test.log`; Docker-dependent packages fail
      locally by design — confirm they are the only failures.
- [x] 3.3 Regenerate `THIRD_PARTY_NOTICES.md`.
- [ ] 3.4 Dev pipeline green, PR merged, prod green.
- [ ] 3.5 `gh api --paginate repos/{owner}/{repo}/dependabot/alerts?state=open`
      returns no `critical`/`high`.
