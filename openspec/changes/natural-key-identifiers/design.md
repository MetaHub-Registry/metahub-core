# Design — natural-key identifiers (metahub-core)

## D1. Natural-key lookups, IDs internal

The rule is narrow: an exported method reached from the public API accepts a
natural key. Once the method has matched that key, it may use `id(n)` for joins,
updates and deletes. The 161 existing `id(...)` occurrences are not rewritten
here; that is the deferred Cypher restructuring.

| Method | Key |
|---|---|
| `GetLayersForManifestDigest` | manifest `digest` |
| `ManipulationsWithConfigByProfileLabel` | profile `label` |
| `OrasArtifactGetByDigest` | `repo` + `digest` |
| `OrasArtifactGetByName` | `repo` + `artifact_type` (tree) |
| `CollectionListingGetByName` | `repo` + `tag` |

## D2. Serialisation hides IDs

`json:"-"` is the enforcement point for the types the API returns directly.
The API also maps some types to DTOs; the tag is the second line of defence.

## D3. No compatibility shim

The consumers are first-party and pre-1.0. The by-ID lookups that the public
API used are gone; the methods that remain by-ID are internal or unused.

## D4. The API switches at the pin

The API pinned the session commit as a pseudo-version and called the new
methods. The release lane has since replaced that pseudo-version with the
released tag `v0.3.12`, which `metahub-api` and `metahub-admin` now consume.
