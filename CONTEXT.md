# Context: metalib

> This file helps AI agents understand the repo quickly.
> Updated after each session that modifies this repo.

## What This Repo Is

The shared Go library for MetaHub. Provides the `magazine.Magazine`
interface, neo4j backend implementation, type definitions, and all
Cypher queries. Both metahub-api and metahub-admin depend on this.

## Architecture

```
pkg/
  backends/
    magazine/              Magazine interface + orchestration
      main.go              Magazine struct and interface definition
      collection.go        Collection/repo operations
      profile.go           Profile operations
      manifest.go          Manifest CRUD
      compute.go           Compute operations (manifestlists, etc.)
      filter.go            Filter logic
      platform.go          Platform matching
    neo/                   Neo4j backend (the main backend)
      collection_listing.go  Collection listing Cypher queries
      collection_tag.go      Collection-by-tag queries
      profiles.go            Profile queries
      rule.go                Rule evaluation queries
      rule_invariants.go     D5 invariant helper
      manifest.go            Manifest CRUD queries
      descriptor.go          Descriptor/referrers queries
      index.go               Index operations
      slot.go                Slot operations
      repo.go                Repo node queries
      filter.go              Filter queries
    minio/                 MinIO blob storage backend
    sqlite/                SQLite backend (legacy, mostly dead)
  types/                   Shared type definitions
    profiles.go            Profile type
    rules.go               Rule type
    collection_listing.go  CollectionListing type
    collection_filter_pair.go  Filter pair for UI dropdowns
    manifest.go            Manifest type (with ContentSize)
    index.go               Index type
    slot.go                Slot type
    repo.go                Repo type
    platform.go            Platform type
  yaml/config/             YAML config parsing (profiles, rules, collections)
  version/                 Version string (bumped by bump2version)
  utils/                   Shared utilities (expr, regex, etc.)
```

## Key Patterns

### Three Graph Patterns

The slot-centred migration left three coexisting patterns. Any query
walking from Repo to Manifest must handle all three:

1. **Legacy**: `(:Repo)-[:UPSTREAM]->(:ManifestWrapper)-[:WRAPS]->(:Manifest)`
2. **Slot-centred**: `(:Repo)-[:HAS_TAG]->(:Tag)-[:POINTS_TO]->(:Index)-[:REFERENCES]->(:Manifest)`
3. **Direct UPSTREAM**: `(:Repo)-[:UPSTREAM]->(:Manifest)`

Pattern 2 is the target. Patterns 1 and 3 are backward compatibility.
See `CollectionListingGetByRepoLike` for the canonical three-branch query.

### Magazine Interface

The `magazine.Magazine` interface is the contract between metahub-api
handlers and the backend. All data access goes through this interface.
The neo4j backend (`pkg/backends/neo/`) is the primary implementation.

### Rule Evaluation

Rules use `expr` expressions evaluated against manifest properties.
`AssertRuleShortcutInvariants` (D5) validates rule-shortcut consistency.

## Testing

```bash
./test.sh 2>&1 > test.log    # full suite with coverage
```

Neo4j tests need a running neo4j instance (see docker-compose or
use the dev environment).

## Recent Changes

- Referrers `:Index` label fix in `descriptor.go`
- Three graph pattern fallback in collection listing/tag queries
- `content_size` (config + layer blobs) added to manifest queries
- Dev versioning with `-devN` suffix
- SonarQube version auto-detection

## Gotchas

- `ManifestWrapper` label still exists for backward compat — don't delete
- `collect(DISTINCT blob)` dedup is critical in content_size queries
- 19 legacy functions still use ManifestWrapper/UPSTREAM patterns
- `compute manifestlists` doesn't produce `:Index` for CE-parsed data
- `artifactType` not populated in referrers response descriptors
