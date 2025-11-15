package cache

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/indy9000/userstore/models"
)

type testUser struct {
	models.BaseUser
	Name string `json:"name"`
}

func newTestUser() *testUser {
	return &testUser{}
}

func TestUserCacheSetAndGet(t *testing.T) {
	dir := t.TempDir()
	c := NewUserCache[*testUser](dir, 0, newTestUser)

	if err := c.Set("user-a", func(u *testUser) {
		u.Name = "alpha"
	}); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	got, err := c.Get("user-a")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Name != "alpha" {
		t.Fatalf("expected cached name alpha, got %s", got.Name)
	}

	// Create a new cache instance to ensure data is loaded from disk.
	c2 := NewUserCache[*testUser](dir, 0, newTestUser)
	got2, err := c2.Get("user-a")
	if err != nil {
		t.Fatalf("Get from new cache failed: %v", err)
	}
	if got2.Name != "alpha" {
		t.Fatalf("expected disk-loaded name alpha, got %s", got2.Name)
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

func TestUserCacheGetCreatesUser(t *testing.T) {
	dir := t.TempDir()
	c := NewUserCache[*testUser](dir, 0, newTestUser)
	user, err := c.Get("new-user")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if user.GetUserID() != "new-user" {
		t.Fatalf("expected user id new-user, got %s", user.GetUserID())
	}
	if user.GetLastUpdated().IsZero() {
		t.Fatalf("expected lastUpdated to be set")
	}
}

func TestUserCacheUpdatePersists(t *testing.T) {
	dir := t.TempDir()
	c := NewUserCache[*testUser](dir, 0, newTestUser)
	if err := c.Set("user", func(u *testUser) {
		u.Name = "before"
	}); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	before, err := c.Get("user")
	if err != nil {
		t.Fatalf("initial Get failed: %v", err)
	}
	beforeTime := before.GetLastUpdated()

	time.Sleep(10 * time.Millisecond)
	if err := c.Update("user", func(u *testUser) {
		u.Name = "after"
	}); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	after, err := c.Get("user")
	if err != nil {
		t.Fatalf("Get after update failed: %v", err)
	}
	if after.Name != "after" {
		t.Fatalf("expected Name after, got %s", after.Name)
	}
	if !after.GetLastUpdated().After(beforeTime) {
		t.Fatalf("expected lastUpdated to advance")
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
	if _, err := c.Get("first"); err != nil {
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
