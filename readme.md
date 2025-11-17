# UserStore

UserStore is a simple concurrent document storage. It has an in-memory write-through LRU cache, a backing store on disk, and built-in concurrency controls so per-user operations run safely in parallel without blocking the entire cache. This can be used to manage user-centric data on your backend VPS with minimal ceremony. Data is isolated in user folders.

## Highlights

- Write-through generics-based cache with configurable LRU eviction.
- Simple JSON storage: every user lives under `baseFolder/<user>/<user>.json`.
- Concurrency-safe operations (`Set`, `View`, `Update`, `Delete`) guarded by RW locks, with per-user mutexes so user-specific work doesn't block the entire cache.
- Deleted users are archived to `baseFolder/deleted/<user>` for later inspection/recovery.
- Minimal interface surface (`BaseUserOps`) so you can adapt existing structs easily.
- Self-reported semantic version via `userstore.Version` so you can assert compatible builds.

## Quick start

1. Define your user model by embedding `models.BaseUser` (or implementing `BaseUserOps` yourself).
2. Create a cache instance by passing the base folder, an optional max capacity, and a constructor for your type.
3. Use the provided helpers to manage user records. The cache only grabs its global RW lock while it touches the shared map/LRU; user-specific reads/writes happen under that user’s mutex so concurrent IDs don’t block each other. `View` and `Update` run your closures while holding the appropriate locks, so they are concurrency-safe out of the box. All persistence is handled by the cache so data survives process restarts.

```go
type Profile struct {
    models.BaseUser
    Email string `json:"email"`
}

func newProfile() *Profile { return &Profile{} }

store := cache.NewUserCache[*Profile]("/var/lib/users", 1000, newProfile)

// Create a user (fails if it already exists on disk or in cache).
err := store.Set("user-123", func(p *Profile) {
    p.Email = "user@example.com"
})

// Read a user (returns ErrUserNotFound if the record is absent).
err = store.View("user-123", func(profile *Profile) error {
    fmt.Printf("Loaded %s with email %s\n", profile.GetUserID(), profile.Email)
    return nil
})

// Update a user and persist the change atomically.
err = store.Update("user-123", func(p *Profile) error {
    p.Email = "new@example.com"
    return nil
})

// Delete removes the entry from the cache and moves its folder to base/deleted/<user>.
err = store.Delete("user-123")
```

`models.BaseUser` still exposes `RLock/RUnlock/Lock/Unlock` in case you need custom coordination, but most callers can stick with the cache helpers (`Set`, `View`, `Update`, `Delete`) and avoid manual locking entirely. Because the cache separates the global lock from per-user locks, work for different users proceeds in parallel even when they inspect or mutate their records. Returning a non-nil error from the `Update` callback automatically rolls the in-memory struct back to the on-disk snapshot, so callers never leave partially mutated objects behind. `View` runs your callback while holding a read lock (and returns `ErrUserNotFound` if the record is absent), so if you need to use the data outside the closure, make an explicit read-only copy.

### Operations at a glance

- `Set(id, initializer)`: creates a brand-new user folder + JSON file; returns `ErrUserExists` if the user already exists.
- `View(id, viewer)`: acquires a read lock, invokes your closure so you can inspect fields without leaking the shared pointer, and returns `ErrUserNotFound` if the user does not exist yet.
- `Update(id, updater)`: locks the user record, runs your mutation, refreshes `lastUpdated`, writes JSON back to disk, and bumps the LRU entry in a single critical section guarded by the user’s mutex (not the entire cache). If the callback returns an error, the struct is reloaded from disk and the error is bubbled up.
- `Delete(id)`: removes the entry from the in-memory cache and moves `baseFolder/<id>` to `baseFolder/deleted/<id>` for archival.
- Automatic LRU eviction keeps the in-memory cache at or below the capacity you configure, while the disk copy is retained.

## Disk layout

After writing a few users, the folder might look like:

```
/var/lib/users
├── deleted
│   └── user-123
│       └── user-123.json
├── user-456
│   └── user-456.json
└── user-789
    └── user-789.json
```

Each user folder contains exactly one JSON file named after the user ID. You can safely back up or replicate the base folder for disaster recovery.

## Error handling tips

- `cache.ErrUserExists` is returned by `Set` if the user already exists in cache or on disk. Use `errors.Is` to check the value.
- Other file-system or JSON errors are propagated directly; they usually warrant logging or bubbling up to the caller.
- `View` returns `ErrUserNotFound` when the user does not exist. Call `Set` up front if you need to guarantee the record is present.

## Testing

The repository ships with table-driven unit tests covering the cache behavior (`go test ./...`). When adding new functionality, extend the test suite so disk interactions remain safe and deterministic.

## Versioning

Import the root module to introspect the published version at runtime:

```go
import "github.com/indy9000/userstore"

fmt.Println("userstore version:", userstore.Version)
```

This value follows semantic versioning. Use it in diagnostics or startup checks to ensure the running binary uses the expected build. Refer to [CHANGELOG.md](./changelog.md) for release notes.

## Runnable example

A fully working sample lives under `examples/basic`. Run it with:

```bash
go run ./examples/basic
```

The program writes user data into a temporary folder, exercises `Set`, `View`, `Update`, and `Delete`, then prints the resulting archive location to illustrate that operations against one user do not block the whole cache.


## Project layout

```
.
├── cache
│   └── user_cache.go     # generic LRU cache with disk backing
├── examples
│   └── basic
│       └── main.go       # runnable demo
├── go.mod
└── models
    └── base_user.go      # BaseUser struct + BaseUserOps interface
```

Import the specific pieces you need:

```go
import (
    "github.com/indy9000/userstore/cache"
    "github.com/indy9000/userstore/models"
)
```

The `models` package contains the base data model and interface that your app-specific
user structs should satisfy. The `cache` package hosts the LRU cache implementation
that uses the interface so you can plug in your own user type.
