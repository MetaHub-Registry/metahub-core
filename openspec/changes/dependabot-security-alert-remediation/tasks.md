# Tasks — remediate the Dependabot backlog

Repository: `metahub-core`. Session provenance: `open-source-metahub`,
2026-10-27. `metahub-core` is checked out as `metalib/` in that session.

## 1. Bumps

- [ ] 1.1 `github.com/containerd/containerd` -> `1.7.29 , 1.7.33` (closes 1 finding(s): high)
- [ ] 1.2 `github.com/containers/image/v5` -> `5.29.3` (closes 1 finding(s): high)
- [ ] 1.3 `github.com/docker/distribution` -> `2.8.2-beta.1` (closes 1 finding(s): high)
- [ ] 1.4 `github.com/docker/docker` -> `25.0.6 , no fix` (closes 2 finding(s): critical, high)
- [ ] 1.5 `github.com/klauspost/compress` -> `1.18.7` (closes 1 finding(s): high)

## 2. No-fix advisories

- [ ] 2.1 Record a dated acceptance in `CONTEXT.md` for each advisory
      whose `first_patched_version` is null (advisory id, reachability
      argument, revisit date).

## 3. Verification

- [ ] 3.1 `GOWORK=off go build ./... && go vet ./...`
- [ ] 3.2 `./test.sh 2>&1 > test.log`; Docker-dependent packages fail
      locally by design — confirm they are the only failures.
- [ ] 3.3 Regenerate `THIRD_PARTY_NOTICES.md`.
- [ ] 3.4 Dev pipeline green, PR merged, prod green.
- [ ] 3.5 `gh api --paginate repos/{owner}/{repo}/dependabot/alerts?state=open`
      returns no `critical`/`high`.
