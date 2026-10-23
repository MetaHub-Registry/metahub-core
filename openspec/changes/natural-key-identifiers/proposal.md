## Why

The core's exported database API is used by the public handlers, and several of
its serialised types still carry Neo4j element IDs. A handler that receives a
node ID cannot be made natural-key-only without a core lookup that takes the
natural key. The IDs are storage details: they change on a graph rebuild and
must not be part of the public contract.

## What Changes

- New lookups: `GetLayersForManifestDigest(ctx, digest)` and
  `ManipulationsWithConfigByProfileLabel(ctx, label)`; the ORAS path gains a
  `repo` + `digest` lookup. No exported lookup reached from the public API
  takes an element ID.
- `OrasArtifact`, `Manipulation`, `ManipuConfig` and `ManipuCommand` hide `ID`
  with `json:"-"`; `www.User.ID` and `www.Secret.ID` do the same.
- `id(n)` stays an internal Cypher detail. The change does not rewrite the
  existing Cypher joins.
- Tests cover the new lookups and the serialisation shape.

## Impact

- Breaking pre-1.0 change for anything that decoded the affected types; the
  only consumers are `metahub-api` (updated in its change) and the admin,
  which pins the same core version but does not call the new lookups.
- `CHANGELOG.md` records the hidden IDs and the new lookup methods.
