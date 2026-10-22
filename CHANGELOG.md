# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
semantic versioning once it reaches 1.0.

## [Unreleased]

### Added

- `GetLayersForManifestDigest`, `ManipulationsWithConfigByProfileLabel` and
  `OrasArtifactGetByDigest`: public callers can address a resource without a
  storage element ID. The by-ID methods remain for internal joins.

### Changed — BREAKING (pre-1.0)

- `types.OrasArtifact`, `types.Manipulation`, `types.ManipuConfig` and
  `types.ManipuCommand` no longer serialise their element ID (`json:"-"`).
  Code that consumed `id` from these JSON payloads must use the natural key.
