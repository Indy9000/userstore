package cache

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/indy9000/userstore/models"
)

type testUser struct {
	models.BaseUser
	Name    string `json:"name"`
	Counter int    `json:"counter"`
}

func newTestUser() *testUser {
	return &testUser{}
}

func TestUserCacheSetAndView(t *testing.T) {
	dir := t.TempDir()
	c := NewUserCache[*testUser](dir, 0, newTestUser)

	if err := c.Set("user-a", func(u *testUser) {
		u.Name = "alpha"
	}); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	if err := c.View("user-a", func(got *testUser) error {
		if got.Name != "alpha" {
			t.Fatalf("expected cached name alpha, got %s", got.Name)
		}
		return nil
	}); err != nil {
		t.Fatalf("View failed: %v", err)
	}

	// Create a new cache instance to ensure data is loaded from disk.
	c2 := NewUserCache[*testUser](dir, 0, newTestUser)
	if err := c2.View("user-a", func(got2 *testUser) error {
		if got2.Name != "alpha" {
			t.Fatalf("expected disk-loaded name alpha, got %s", got2.Name)
		}
		return nil
	}); err != nil {
		t.Fatalf("View from new cache failed: %v", err)
	}
}

func TestUserCacheSetDuplicate(t *testing.T) {
	dir := t.TempDir()
	c := NewUserCache[*testUser](dir, 0, newTestUser)
	if err := c.Set("dup", func(u *testUser) {}); err != nil {
		t.Fatalf("initial Set failed: %v", err)
	}
	err := c.Set("dup", func(u *testUser) {})
	if !errors.Is(err, ErrUserExists) {
		t.Fatalf("expected ErrUserExists, got %v", err)
	}
}

func TestUserCacheViewMissingReturnsError(t *testing.T) {
	dir := t.TempDir()
	c := NewUserCache[*testUser](dir, 0, newTestUser)
	err := c.View("new-user", func(user *testUser) error {
		t.Fatalf("viewer should not run for missing user")
		return nil
	})
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserCacheUpdateWritesChanges(t *testing.T) {
	dir := t.TempDir()
	c := NewUserCache[*testUser](dir, 0, newTestUser)
	if err := c.Set("user", func(u *testUser) {
		u.Name = "before"
	}); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	var beforeTime time.Time
	if err := c.View("user", func(u *testUser) error {
		beforeTime = u.GetLastUpdated()
		return nil
	}); err != nil {
		t.Fatalf("initial View failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)
	if err := c.Update("user", func(u *testUser) error {
		u.Name = "after"
		return nil
	}); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if err := c.View("user", func(after *testUser) error {
		if after.Name != "after" {
			t.Fatalf("expected Name after, got %s", after.Name)
		}
		if !after.GetLastUpdated().After(beforeTime) {
			t.Fatalf("expected lastUpdated to advance")
		}
		return nil
	}); err != nil {
		t.Fatalf("View after update failed: %v", err)
	}
}

func TestUserCacheUpdateRollsBackOnError(t *testing.T) {
	dir := t.TempDir()
	c := NewUserCache[*testUser](dir, 0, newTestUser)
	if err := c.Set("user", func(u *testUser) {
		u.Name = "before"
	}); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	wantErr := errors.New("boom")
	err := c.Update("user", func(u *testUser) error {
		u.Name = "after"
		return wantErr
	})
	if err == nil || err.Error() != wantErr.Error() {
		t.Fatalf("expected error %v, got %v", wantErr, err)
	}

	if err := c.View("user", func(user *testUser) error {
		if user.Name != "before" {
			t.Fatalf("expected user to be rolled back to before, got %s", user.Name)
		}
		return nil
	}); err != nil {
		t.Fatalf("View failed: %v", err)
	}
}

func TestUserCacheUpdateMissingUser(t *testing.T) {
	dir := t.TempDir()
	c := NewUserCache[*testUser](dir, 0, newTestUser)
	err := c.Update("does-not-exist", func(u *testUser) error {
		return nil
	})
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserCacheEvictsLeastRecentlyUsed(t *testing.T) {
	dir := t.TempDir()
	c := NewUserCache[*testUser](dir, 1, newTestUser)
	for _, id := range []string{"first", "second"} {
		id := id
		if err := c.Set(id, func(u *testUser) {
			u.Name = id
		}); err != nil {
			t.Fatalf("Set %s failed: %v", id, err)
		}
	}

	if _, ok := c.cache["first"]; ok {
		t.Fatalf("expected first entry to be evicted from cache map")
	}
	if c.lru.Len() != 1 {
		t.Fatalf("expected lru length 1, got %d", c.lru.Len())
	}

	// Data should still exist on disk.
	if err := c.View("first", func(*testUser) error { return nil }); err != nil {
		t.Fatalf("expected evicted item to be recoverable from disk, got %v", err)
	}
}

func TestUserCacheStoresInUserFolders(t *testing.T) {
	dir := t.TempDir()
	c := NewUserCache[*testUser](dir, 0, newTestUser)
	if err := c.Set("user-folder", func(u *testUser) {
		u.Name = "folder"
	}); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	fp := filepath.Join(dir, "user-folder", "user-folder.json")
	if _, err := os.Stat(fp); err != nil {
		t.Fatalf("expected file to exist at %s: %v", fp, err)
	}
}

// Adversarial: hammers one user with concurrent Updates. Under -race this
// caught the unsynchronized read of entry.value in getOrLoad's cache-hit path
// racing the write in persistLocked; it also guards against lost updates.
func TestUserCacheConcurrentUpdateSameUser(t *testing.T) {
	dir := t.TempDir()
	c := NewUserCache[*testUser](dir, 0, newTestUser)
	if err := c.Set("hot-user", func(u *testUser) {}); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	const n = 100
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.Update("hot-user", func(u *testUser) error {
				u.Counter++
				return nil
			}); err != nil {
				t.Errorf("Update failed: %v", err)
			}
		}()
		// interleave readers to also exercise View vs Update on the same entry
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.View("hot-user", func(u *testUser) error {
				_ = u.Counter
				return nil
			}); err != nil {
				t.Errorf("View failed: %v", err)
			}
		}()
	}
	wg.Wait()

	if err := c.View("hot-user", func(u *testUser) error {
		if u.Counter != n {
			t.Fatalf("lost updates: got %d, want %d", u.Counter, n)
		}
		return nil
	}); err != nil {
		t.Fatalf("final View failed: %v", err)
	}
}

// Happy path: concurrent Updates spread across distinct users all land
// correctly in memory and on disk.
func TestUserCacheConcurrentUpdateDistinctUsers(t *testing.T) {
	dir := t.TempDir()
	c := NewUserCache[*testUser](dir, 0, newTestUser)

	userIds := []string{"user-1", "user-2", "user-3", "user-4"}
	for _, id := range userIds {
		if err := c.Set(id, func(u *testUser) {}); err != nil {
			t.Fatalf("Set %s failed: %v", id, err)
		}
	}

	const perUser = 25
	var wg sync.WaitGroup
	for _, id := range userIds {
		for i := 0; i < perUser; i++ {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				if err := c.Update(id, func(u *testUser) error {
					u.Counter++
					u.Name = id
					return nil
				}); err != nil {
					t.Errorf("Update %s failed: %v", id, err)
				}
			}(id)
		}
	}
	wg.Wait()

	// verify via a fresh cache so the values are read back from disk
	c2 := NewUserCache[*testUser](dir, 0, newTestUser)
	for _, id := range userIds {
		if err := c2.View(id, func(u *testUser) error {
			if u.Counter != perUser {
				t.Fatalf("%s: got %d, want %d", id, u.Counter, perUser)
			}
			if u.Name != id {
				t.Fatalf("%s: got name %s", id, u.Name)
			}
			return nil
		}); err != nil {
			t.Fatalf("View %s failed: %v", id, err)
		}
	}
}

func TestUserCacheDeleteMovesToDeletedFolder(t *testing.T) {
	dir := t.TempDir()
	c := NewUserCache[*testUser](dir, 0, newTestUser)
	if err := c.Set("to-delete", func(u *testUser) {
		u.Name = "bye"
	}); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	if err := c.Delete("to-delete"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	originalDir := filepath.Join(dir, "to-delete")
	if _, err := os.Stat(originalDir); !os.IsNotExist(err) {
		t.Fatalf("expected original folder %s to be gone, err=%v", originalDir, err)
	}

	deletedFile := filepath.Join(dir, "deleted", "to-delete", "to-delete.json")
	if _, err := os.Stat(deletedFile); err != nil {
		t.Fatalf("expected file to be moved to %s: %v", deletedFile, err)
	}
}
