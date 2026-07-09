# Agent Instructions

## Codebase Knowledge Graph

This repo ships a pre-built knowledge graph of its own source at
`.understand-anything/knowledge-graph.json` (nodes, edges, architectural layers,
and a guided tour). **Consult it before broad code exploration** — it is faster
and cheaper than blind file/grep search for "where does X live", "who calls X",
or "what depends on Y".

### How to use it

- **First stop for orientation.** Grep the graph for relevant `name`/`summary`/`tags`
  to find nodes, then follow `imports`/`calls`/`contains` edges for the 1-hop
  subgraph. Don't dump the whole file into context — search the sections you need.
- **Resolve symbols via the graph, not `grep`.** Node IDs are
  `function:<relpath>:<Name>` (methods likewise) and `calls` edges run
  caller → callee, so "who uses this function?" is a graph query — not a source
  grep. Reserve `rg`/grep for string/literal search (replace `SplitImageName`
  with the symbol you're chasing):

  ```bash
  KG=.understand-anything/knowledge-graph.json

  # Where is it defined?
  jq -r '.nodes[] | select(.id|endswith(":SplitImageName")) | .id' "$KG"

  # Who calls it? (the usages you'd otherwise grep for)
  jq -r '.edges[] | select(.type=="calls" and (.target|endswith(":SplitImageName"))) | .source' "$KG"

  # What does it call / import? (1-hop out)
  jq -r '.edges[] | select((.type=="calls" or .type=="imports") and (.source|endswith(":SplitImageName"))) | .target' "$KG"
  ```
- **Scope.** The graph is scoped to code + key infra (see
  `.understand-anything/.understandignore`); OpenSpec docs and misc yaml are
  intentionally excluded.

### Keeping it fresh

The graph is tied to a git commit (`.understand-anything/meta.json`) with a
fingerprints baseline. After material code changes, regenerate it with the
global `/understand` tool (re-analyzes only changed files) and commit the updated
`.understand-anything/` artifacts alongside the code so the graph's commit hash
tracks `HEAD`. A stale-but-present graph is still a useful map; note staleness if
the commit hash differs from `HEAD`.

## After Each Change

At the end of every change, run the following two scripts in order:

1. **Tests** — run the full test suite with coverage:
   ```
   ./test.sh
   ```

2. **SonarQube analysis** — submit coverage results to SonarQube:
   ```
   ./misc/bin/sonar.sh
   ```

Both scripts must pass before a change is considered complete.
