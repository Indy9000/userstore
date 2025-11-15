package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/indy9000/userstore/cache"
	"github.com/indy9000/userstore/models"
)

type profile struct {
	models.BaseUser
	Email string `json:"email"`
}

func newProfile() *profile { return &profile{} }

func main() {
	baseDir := filepath.Join(os.TempDir(), "userstore-example")
	if err := os.RemoveAll(baseDir); err != nil {
		log.Fatalf("failed to clean example dir: %v", err)
	}

	store := cache.NewUserCache[*profile](baseDir, 2, newProfile)

	if err := store.Set("alice", func(p *profile) {
		p.Email = "alice@example.com"
	}); err != nil {
		log.Fatalf("set: %v", err)
	}

	alice, err := store.Get("alice")
	if err != nil {
		log.Fatalf("get: %v", err)
	}
	fmt.Printf("Loaded %s email=%s\n", alice.GetUserID(), alice.Email)

	if err := store.Update("alice", func(p *profile) {
		p.Email = "new-alice@example.com"
	}); err != nil {
		log.Fatalf("update: %v", err)
	}

	updated, err := store.Get("alice")
	if err != nil {
		log.Fatalf("get after update: %v", err)
	}
	fmt.Printf("Updated %s email=%s lastUpdated=%s\n", updated.GetUserID(), updated.Email, updated.GetLastUpdated().Format(time.RFC3339))

	if err := store.Delete("alice"); err != nil {
		log.Fatalf("delete: %v", err)
	}

	fmt.Printf("Deleted user archived at %s\n", filepath.Join(baseDir, "deleted", "alice"))
}
