# UserStore

UserStore is a simple document storage. It has an in memory write-through LRU cache and a backing storage to disk.
This can be used to manage user centric data on your backend VPS with minimal ceremony.
Data is isolated in user folders.

## Highlights

- Write-through generics-based cache with configurable LRU eviction.
- Simple JSON storage: every user lives under `baseFolder/<user>/<user>.json`.
- Concurrency-safe operations (`Set`, `Get`, `Update`, `Delete`) guarded by RW locks.
- Deleted users are archived to `baseFolder/deleted/<user>` for later inspection/recovery.
- Minimal interface surface (`BaseUserOps`) so you can adapt existing structs easily.

## Quick start

1. Define your user model by embedding `models.BaseUser` (or implementing `BaseUserOps` yourself).
2. Create a cache instance by passing the base folder, an optional max capacity, and a constructor for your type.
3. Use the provided CRUD helpers to manage user records. All operations persist to disk so data survives process restarts.

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

// Read a user. If it does not yet exist on disk, an empty record is initialized.
profile, err := store.Get("user-123")

// Update a user and immediately persist the change.
err = store.Update("user-123", func(p *Profile) {
    p.Email = "new@example.com"
})

// Delete removes the entry from the cache and moves its folder to base/deleted/<user>.
err = store.Delete("user-123")
```

### Operations at a glance

- `Set(id, initializer)`: creates a brand-new user folder + JSON file; returns `ErrUserExists` if the user already exists.
- `Get(id)`: returns the cached struct, rehydrating from `baseFolder/<id>/<id>.json`; creates a new empty user if no file is present.
- `Update(id, updater)`: lets you mutate the struct and automatically writes the JSON back to disk in a single critical section.
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
- `Get` always returns a user struct (either rehydrated from disk or newly initialized). If you do not want on-demand creation, call a helper like `exists` yourself before `Get`.

## Testing

The repository ships with table-driven unit tests covering the cache behavior (`go test ./...`). When adding new functionality, extend the test suite so disk interactions remain safe and deterministic.

## Runnable example

A fully working sample lives under `examples/basic`. Run it with:

```bash
go run ./examples/basic
```

The program writes user data into a temporary folder, exercises `Set`, `Get`, `Update`, and `Delete`, then prints the resulting archive location.


## Project layout

```
.
├── go.mod
└── pkg
    ├── cache
    │   └── user_cache.go     # generic LRU cache with disk backing
    └── models
        └── base_user.go      # BaseUser struct + BaseUserOps interface
```

Import the specific pieces you need:

```go
import (
    "userstore/pkg/cache"
    "userstore/pkg/models"
)
```

The `models` package contains the base data model and interface that your app-specific
user structs should satisfy. The `cache` package hosts the LRU cache implementation
that uses the interface so you can plug in your own user type.
