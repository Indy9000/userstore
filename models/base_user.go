package models

import (
	"sync"
	"time"
)

// BaseUser holds the minimum metadata persisted for every user.
type BaseUser struct {
	mu          sync.RWMutex
	UserID      string    `json:"userId"`
	LastUpdated time.Time `json:"lastUpdated"`
}

// BaseUserOps is an interface that all app-specific user types must implement.
// It embeds the common fields via methods for flexibility.
type BaseUserOps interface {
	GetUserID() string
	SetUserID(string)
	GetLastUpdated() time.Time
	SetLastUpdated()
	RLock()
	RUnlock()
	Lock()
	Unlock()
}

// Ensure BaseUser implements BaseUserOps.
var _ BaseUserOps = (*BaseUser)(nil)

func (b *BaseUser) GetUserID() string         { return b.UserID }
func (b *BaseUser) SetUserID(id string)       { b.UserID = id }
func (b *BaseUser) GetLastUpdated() time.Time { return b.LastUpdated }
func (b *BaseUser) SetLastUpdated()           { b.LastUpdated = time.Now().UTC() }
func (b *BaseUser) RLock()                    { b.mu.RLock() }
func (b *BaseUser) RUnlock()                  { b.mu.RUnlock() }
func (b *BaseUser) Lock()                     { b.mu.Lock() }
func (b *BaseUser) Unlock()                   { b.mu.Unlock() }
