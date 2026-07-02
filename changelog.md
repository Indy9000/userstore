# Changelog

All notable changes to this project will be documented in this file. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.4.1] - 2026-07-02

### Fixed

- Data race in `getOrLoad`'s cache-hit path: the LRU entry value was read after releasing the cache mutex while `persistLocked` reassigns it under the mutex, tripping `go test -race` on concurrent same-user `Update`/`View`. The value is now read under the lock.

### Added

- Concurrency tests: an adversarial test hammering one user with concurrent `Update`/`View` calls (reproduces the race pre-fix and asserts no lost updates) and a happy-path test of concurrent updates across distinct users verified from disk.

## [0.4.0] - 2024-02-15

### Added

- `View` helper that acquires a read lock and runs a callback so callers inspect user data without leaking the shared pointer; the callback now receives `nil` when the user does not exist.
- Internal `getOrLoad` helper shared by `View`/`Update`.
- Documentation clarifying how to copy data out of `View`.
- Convenience scripts `unit-test.sh` and `race-test.sh` to run the test suite locally.

### Changed

- Removed the exported `Get` method in favor of `View`.
- `View` no longer auto-creates empty records for missing users; `Update` now returns `ErrUserNotFound` when the target is absent.
- Updated tests, README, and examples to use the closure-based API.
- README introduction now calls out the concurrent design explicitly.
- Bumped module version to `v0.4.0`.

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

[0.4.1]: https://github.com/indy9000/userstore/releases/tag/v0.4.1
[0.4.0]: https://github.com/indy9000/userstore/releases/tag/v0.4.0
[0.3.0]: https://github.com/indy9000/userstore/releases/tag/v0.3.0
[0.2.0]: https://github.com/indy9000/userstore/releases/tag/v0.2.0
[0.1.0]: https://github.com/indy9000/userstore/releases/tag/v0.1.0
