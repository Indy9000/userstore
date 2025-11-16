# Changelog

All notable changes to this project will be documented in this file. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.0] - 2024-02-14

### Added

- `Update` callback now accepts `func(T) error)` and automatically rolls back in-memory state when a non-nil error is returned.
- `restoreFromDisk` helper to reload JSON into an existing cached struct.
- Documentation describing the rollback semantics and concurrency model.
- Changelog file to track future releases.

### Changed

- Bumped module version to `v0.3.0`.
- Updated examples and README to reflect the new `Update` signature.

## [0.2.0] - 2024-02-13

### Added

- Introduced per-user locking in `BaseUser` + `UserCache` to avoid blocking the entire cache for user-specific work.
- Added `Persist` helper (later superseded by the improved `Update`).
- User-facing documentation for concurrency behavior.

## [0.1.0] - 2024-02-10

### Added

- Initial release: generic `UserCache`, disk-backed persistence, and base models.

[0.3.0]: https://github.com/indy9000/userstore/releases/tag/v0.3.0
[0.2.0]: https://github.com/indy9000/userstore/releases/tag/v0.2.0
[0.1.0]: https://github.com/indy9000/userstore/releases/tag/v0.1.0
