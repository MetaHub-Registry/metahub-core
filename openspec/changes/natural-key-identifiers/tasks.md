# Tasks — natural-key identifiers (metahub-core)

- [x] 1.1 Add `GetLayersForManifestDigest(ctx, digest)`; the manifest already
      carries its digest.
- [x] 1.2 Add `ManipulationsWithConfigByProfileLabel(ctx, label)`.
- [x] 1.3 Add a `repo` + `digest` ORAS lookup (`OrasArtifactGetByDigest`) and
      keep `OrasArtifactGetByName` for tree navigation.
- [x] 1.4 Confirm no exported lookup reached from the public API takes an
      element ID; `id(n)` remains internal.
- [x] 2.1 Add `json:"-"` to `OrasArtifact.ID`, `Manipulation.ID`,
      `ManipuConfig.ID` and `ManipuCommand.ID`.
- [x] 2.2 Add `json:"-"` to `www.User.ID` and `www.Secret.ID`.
- [x] 3.1 Tests for the three lookups, including the digest-only layer fetch.
- [x] 3.2 Serialisation tests asserting the affected types carry no `id` key.
- [x] 4.1 Record the change in `CHANGELOG.md`.
- [x] 4.2 Full suite and the fast lane green.
